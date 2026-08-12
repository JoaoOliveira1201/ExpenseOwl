package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type PostgresStore struct{ db *sql.DB }

func InitializeStorage() (*PostgresStore, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" && os.Getenv("PGHOST") == "" {
		databaseURL = "postgres://expenseowl:expenseowl@localhost:5432/expenseowl?sslmode=disable"
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	store := &PostgresStore{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate PostgreSQL: %w", err)
	}
	return store, nil
}

func (store *PostgresStore) Close() error { return store.db.Close() }

func (store *PostgresStore) migrate(ctx context.Context) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS expenses (
			id TEXT PRIMARY KEY,
			recurring_id TEXT,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			amount NUMERIC(14,2) NOT NULL,
			date TIMESTAMPTZ NOT NULL,
			owner TEXT NOT NULL DEFAULT 'common',
			notes TEXT NOT NULL DEFAULT '',
			receipt TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS recurring_expenses (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			amount NUMERIC(14,2) NOT NULL,
			category TEXT NOT NULL,
			start_date TIMESTAMPTZ NOT NULL,
			interval TEXT NOT NULL,
			occurrences INTEGER NOT NULL,
			owner TEXT NOT NULL DEFAULT 'common',
			notes TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS app_config (
			id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			categories JSONB NOT NULL,
			category_targets JSONB NOT NULL DEFAULT '{}'
		)`,
		`DO $$ BEGIN
			IF to_regclass('public.config') IS NOT NULL THEN
				INSERT INTO app_config (id, categories, category_targets)
				SELECT 1, categories::jsonb, COALESCE(NULLIF(category_targets, ''), '{}')::jsonb FROM config WHERE id = 'default'
				ON CONFLICT (id) DO NOTHING;
			END IF;
		END $$`,
		`DROP TABLE IF EXISTS config`,
		`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS owner TEXT NOT NULL DEFAULT 'common'`,
		`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS receipt TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE expenses DROP COLUMN IF EXISTS tags`,
		`ALTER TABLE expenses DROP COLUMN IF EXISTS currency`,
		`UPDATE expenses SET owner='common' WHERE owner IS NULL`,
		`ALTER TABLE expenses ALTER COLUMN owner SET DEFAULT 'common'`,
		`ALTER TABLE expenses ALTER COLUMN owner SET NOT NULL`,
		`ALTER TABLE expenses ALTER COLUMN amount TYPE NUMERIC(14,2)`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS owner TEXT NOT NULL DEFAULT 'common'`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE recurring_expenses DROP COLUMN IF EXISTS tags`,
		`ALTER TABLE recurring_expenses DROP COLUMN IF EXISTS currency`,
		`UPDATE recurring_expenses SET owner='common' WHERE owner IS NULL`,
		`ALTER TABLE recurring_expenses ALTER COLUMN owner SET DEFAULT 'common'`,
		`ALTER TABLE recurring_expenses ALTER COLUMN owner SET NOT NULL`,
		`ALTER TABLE recurring_expenses ALTER COLUMN amount TYPE NUMERIC(14,2)`,
		`CREATE INDEX IF NOT EXISTS expenses_date_idx ON expenses (date DESC)`,
		`CREATE INDEX IF NOT EXISTS expenses_owner_date_idx ON expenses (owner, date DESC)`,
		`CREATE INDEX IF NOT EXISTS expenses_recurring_idx ON expenses (recurring_id) WHERE recurring_id IS NOT NULL`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	categories, err := json.Marshal(defaultCategories)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_config (id, categories) VALUES (1, $1) ON CONFLICT (id) DO NOTHING`, categories); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *PostgresStore) GetConfig(ctx context.Context) (Config, error) {
	var categoriesJSON, targetsJSON []byte
	if err := store.db.QueryRowContext(ctx, `SELECT categories, category_targets FROM app_config WHERE id = 1`).Scan(&categoriesJSON, &targetsJSON); err != nil {
		return Config{}, err
	}
	config := Config{Currency: Currency, StartDate: StartDate, CategoryTargets: map[string]float64{}}
	if err := json.Unmarshal(categoriesJSON, &config.Categories); err != nil {
		return Config{}, fmt.Errorf("decode categories: %w", err)
	}
	if err := json.Unmarshal(targetsJSON, &config.CategoryTargets); err != nil {
		return Config{}, fmt.Errorf("decode targets: %w", err)
	}
	return config, nil
}

func (store *PostgresStore) UpdateCategories(ctx context.Context, categories []string) error {
	data, err := json.Marshal(categories)
	if err != nil {
		return err
	}
	_, err = store.db.ExecContext(ctx, `UPDATE app_config SET categories = $1 WHERE id = 1`, data)
	return err
}

func (store *PostgresStore) UpdateCategoryTargets(ctx context.Context, targets map[string]float64) error {
	data, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	_, err = store.db.ExecContext(ctx, `UPDATE app_config SET category_targets = $1 WHERE id = 1`, data)
	return err
}

const expenseColumns = `id, recurring_id, name, category, amount, date, owner, notes, receipt`

func scanExpense(scanner interface{ Scan(...any) error }) (Expense, error) {
	var expense Expense
	var recurringID sql.NullString
	err := scanner.Scan(&expense.ID, &recurringID, &expense.Name, &expense.Category, &expense.Amount, &expense.Date, &expense.Owner, &expense.Notes, &expense.Receipt)
	if recurringID.Valid {
		expense.RecurringID = recurringID.String
	}
	return expense, err
}

func (store *PostgresStore) GetExpenses(ctx context.Context, filter ExpenseFilter) ([]Expense, error) {
	clauses, args := []string{"TRUE"}, []any{}
	if filter.From != nil {
		args = append(args, *filter.From)
		clauses = append(clauses, fmt.Sprintf("date >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		clauses = append(clauses, fmt.Sprintf("date < $%d", len(args)))
	}
	if filter.Owner != "" {
		args = append(args, normalizeOwner(filter.Owner))
		clauses = append(clauses, fmt.Sprintf("owner = $%d", len(args)))
	}
	query := `SELECT ` + expenseColumns + ` FROM expenses WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY date DESC`
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expenses := make([]Expense, 0)
	for rows.Next() {
		expense, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, expense)
	}
	return expenses, rows.Err()
}

func (store *PostgresStore) GetExpense(ctx context.Context, id string) (Expense, error) {
	expense, err := scanExpense(store.db.QueryRowContext(ctx, `SELECT `+expenseColumns+` FROM expenses WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Expense{}, fmt.Errorf("expense not found")
	}
	return expense, err
}

func prepareExpense(expense Expense) Expense {
	if expense.ID == "" {
		expense.ID = uuid.NewString()
	}
	if expense.Owner == "" {
		expense.Owner = "common"
	}
	return expense
}

func (store *PostgresStore) AddExpense(ctx context.Context, expense Expense) (Expense, error) {
	expense = prepareExpense(expense)
	_, err := store.db.ExecContext(ctx, `INSERT INTO expenses (`+expenseColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		expense.ID, nullable(expense.RecurringID), expense.Name, expense.Category, expense.Amount, expense.Date, expense.Owner, expense.Notes, expense.Receipt)
	return expense, err
}

func (store *PostgresStore) AddExpenses(ctx context.Context, expenses []Expense) (int, error) {
	if len(expenses) == 0 {
		return 0, nil
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx, `INSERT INTO expenses (`+expenseColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	defer statement.Close()
	inserted := 0
	for _, expense := range expenses {
		expense = prepareExpense(expense)
		result, err := statement.ExecContext(ctx, expense.ID, nullable(expense.RecurringID), expense.Name, expense.Category, expense.Amount, expense.Date, expense.Owner, expense.Notes, expense.Receipt)
		if err != nil {
			return 0, err
		}
		rows, _ := result.RowsAffected()
		inserted += int(rows)
	}
	return inserted, tx.Commit()
}

func (store *PostgresStore) UpdateExpense(ctx context.Context, id string, expense Expense) (Expense, error) {
	expense.Owner = normalizeOwner(expense.Owner)
	result, err := store.db.ExecContext(ctx, `UPDATE expenses SET recurring_id=$1,name=$2,category=$3,amount=$4,date=$5,owner=$6,notes=$7,receipt=$8 WHERE id=$9`,
		nullable(expense.RecurringID), expense.Name, expense.Category, expense.Amount, expense.Date, expense.Owner, expense.Notes, expense.Receipt, id)
	if err != nil {
		return Expense{}, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return Expense{}, fmt.Errorf("expense not found")
	}
	expense.ID = id
	return expense, nil
}

func (store *PostgresStore) RemoveExpense(ctx context.Context, id string) error {
	result, err := store.db.ExecContext(ctx, `DELETE FROM expenses WHERE id=$1`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("expense not found")
	}
	return nil
}

func (store *PostgresStore) RemoveExpenses(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := store.db.ExecContext(ctx, `DELETE FROM expenses WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

const recurringColumns = `id, name, amount, category, start_date, interval, occurrences, owner, notes`

func scanRecurring(scanner interface{ Scan(...any) error }) (RecurringExpense, error) {
	var recurring RecurringExpense
	err := scanner.Scan(&recurring.ID, &recurring.Name, &recurring.Amount, &recurring.Category, &recurring.StartDate, &recurring.Interval, &recurring.Occurrences, &recurring.Owner, &recurring.Notes)
	return recurring, err
}

func (store *PostgresStore) GetRecurringExpenses(ctx context.Context) ([]RecurringExpense, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT `+recurringColumns+` FROM recurring_expenses ORDER BY start_date, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RecurringExpense, 0)
	for rows.Next() {
		item, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (store *PostgresStore) AddRecurringExpense(ctx context.Context, recurring RecurringExpense) (RecurringExpense, error) {
	if recurring.ID == "" {
		recurring.ID = uuid.NewString()
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return RecurringExpense{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO recurring_expenses (`+recurringColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, recurring.ID, recurring.Name, recurring.Amount, recurring.Category, recurring.StartDate, recurring.Interval, recurring.Occurrences, recurring.Owner, recurring.Notes)
	if err != nil {
		return RecurringExpense{}, err
	}
	if err := insertGenerated(ctx, tx, generateExpenses(recurring, false)); err != nil {
		return RecurringExpense{}, err
	}
	return recurring, tx.Commit()
}

func (store *PostgresStore) UpdateRecurringExpense(ctx context.Context, id string, recurring RecurringExpense, updateAll bool) error {
	recurring.ID = id
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE recurring_expenses SET name=$1,amount=$2,category=$3,start_date=$4,interval=$5,occurrences=$6,owner=$7,notes=$8 WHERE id=$9`, recurring.Name, recurring.Amount, recurring.Category, recurring.StartDate, recurring.Interval, recurring.Occurrences, recurring.Owner, recurring.Notes, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("recurring expense not found")
	}
	if updateAll {
		_, err = tx.ExecContext(ctx, `DELETE FROM expenses WHERE recurring_id=$1`, id)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM expenses WHERE recurring_id=$1 AND date >= $2`, id, time.Now())
	}
	if err != nil {
		return err
	}
	if err := insertGenerated(ctx, tx, generateExpenses(recurring, !updateAll)); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *PostgresStore) RemoveRecurringExpense(ctx context.Context, id string, removeAll bool) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM recurring_expenses WHERE id=$1`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("recurring expense not found")
	}
	if removeAll {
		_, err = tx.ExecContext(ctx, `DELETE FROM expenses WHERE recurring_id=$1`, id)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM expenses WHERE recurring_id=$1 AND date >= $2`, id, time.Now())
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func insertGenerated(ctx context.Context, tx *sql.Tx, expenses []Expense) error {
	statement, err := tx.PrepareContext(ctx, `INSERT INTO expenses (`+expenseColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, expense := range expenses {
		if _, err := statement.ExecContext(ctx, expense.ID, expense.RecurringID, expense.Name, expense.Category, expense.Amount, expense.Date, expense.Owner, expense.Notes, expense.Receipt); err != nil {
			return err
		}
	}
	return nil
}

func generateExpenses(recurring RecurringExpense, futureOnly bool) []Expense {
	date := recurring.StartDate
	remaining := recurring.Occurrences
	if futureOnly {
		for date.Before(time.Now()) && remaining > 0 {
			date = nextOccurrence(date, recurring.Interval)
			remaining--
		}
	}
	expenses := make([]Expense, 0, remaining)
	for range remaining {
		expenses = append(expenses, Expense{ID: uuid.NewString(), RecurringID: recurring.ID, Name: recurring.Name, Category: recurring.Category, Amount: recurring.Amount, Date: date, Owner: recurring.Owner, Notes: recurring.Notes})
		date = nextOccurrence(date, recurring.Interval)
	}
	return expenses
}

func nextOccurrence(date time.Time, interval string) time.Time {
	switch interval {
	case "daily":
		return date.AddDate(0, 0, 1)
	case "weekly":
		return date.AddDate(0, 0, 7)
	case "monthly":
		return date.AddDate(0, 1, 0)
	default:
		return date.AddDate(1, 0, 0)
	}
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
