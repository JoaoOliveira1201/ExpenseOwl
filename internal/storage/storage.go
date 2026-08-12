package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	Currency  = "EUR"
	StartDate = 1
)

var defaultCategories = []string{
	"Food", "Groceries", "Travel", "Rent", "Utilities",
	"Entertainment", "Healthcare", "Shopping", "Miscellaneous", "Income",
}

// Store is the persistence contract used by the HTTP layer. ExpenseOwl has one
// supported backend: PostgreSQL.
type Store interface {
	Close() error
	GetConfig(context.Context) (Config, error)
	UpdateCategories(context.Context, []string) error
	UpdateCategoryTargets(context.Context, map[string]float64) error

	GetExpenses(context.Context, ExpenseFilter) ([]Expense, error)
	GetExpense(context.Context, string) (Expense, error)
	AddExpense(context.Context, Expense) (Expense, error)
	AddExpenses(context.Context, []Expense) (int, error)
	UpdateExpense(context.Context, string, Expense) (Expense, error)
	RemoveExpense(context.Context, string) error
	RemoveExpenses(context.Context, []string) error

	GetRecurringExpenses(context.Context) ([]RecurringExpense, error)
	AddRecurringExpense(context.Context, RecurringExpense) (RecurringExpense, error)
	UpdateRecurringExpense(context.Context, string, RecurringExpense, bool) error
	RemoveRecurringExpense(context.Context, string, bool) error
}

type Config struct {
	Categories      []string           `json:"categories"`
	CategoryTargets map[string]float64 `json:"categoryTargets"`
	Currency        string             `json:"currency"`
	StartDate       int                `json:"startDate"`
}

type ExpenseFilter struct {
	From  *time.Time
	To    *time.Time
	Owner string
}

type Expense struct {
	ID          string    `json:"id"`
	RecurringID string    `json:"recurringID,omitempty"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Date        time.Time `json:"date"`
	Owner       string    `json:"owner"`
	Notes       string    `json:"notes,omitempty"`
	Receipt     string    `json:"receipt,omitempty"`
}

type RecurringExpense struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	StartDate   time.Time `json:"startDate"`
	Interval    string    `json:"interval"`
	Occurrences int       `json:"occurrences"`
	Owner       string    `json:"owner"`
	Notes       string    `json:"notes,omitempty"`
}

var (
	invalidChars   = regexp.MustCompile(`[^\p{L}\p{N}\s.,\-'_!"]`)
	repeatingSpace = regexp.MustCompile(`[ \t]+`)
)

func SanitizeString(value string) string {
	value = invalidChars.ReplaceAllString(value, " ")
	return strings.TrimSpace(repeatingSpace.ReplaceAllString(value, " "))
}

func SanitizeNotes(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for index := range lines {
		lines[index] = SanitizeString(lines[index])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func ValidateCategory(category string) (string, error) {
	category = SanitizeString(category)
	if category == "" {
		return "", fmt.Errorf("category cannot be empty")
	}
	return category, nil
}

func (expense *Expense) Validate() error {
	expense.Name = SanitizeString(expense.Name)
	expense.Category = SanitizeString(expense.Category)
	expense.Owner = normalizeOwner(expense.Owner)
	expense.Notes = SanitizeNotes(expense.Notes)
	if expense.Name == "" || expense.Category == "" {
		return fmt.Errorf("name and category are required")
	}
	if expense.Amount == 0 {
		return fmt.Errorf("amount cannot be zero")
	}
	if expense.Date.IsZero() {
		return fmt.Errorf("date is required")
	}
	if len(expense.Notes) > 2000 {
		return fmt.Errorf("notes cannot exceed 2000 characters")
	}
	if expense.Receipt != "" {
		name := strings.TrimPrefix(expense.Receipt, "/receipts/")
		if name == expense.Receipt || name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
			return fmt.Errorf("invalid receipt reference")
		}
	}
	return nil
}

func (recurring *RecurringExpense) Validate() error {
	recurring.Name = SanitizeString(recurring.Name)
	recurring.Category = SanitizeString(recurring.Category)
	recurring.Owner = normalizeOwner(recurring.Owner)
	recurring.Notes = SanitizeNotes(recurring.Notes)
	if recurring.Name == "" || recurring.Category == "" {
		return fmt.Errorf("name and category are required")
	}
	if recurring.Amount == 0 {
		return fmt.Errorf("amount cannot be zero")
	}
	if recurring.StartDate.IsZero() {
		return fmt.Errorf("start date is required")
	}
	if recurring.Occurrences < 2 {
		return fmt.Errorf("at least 2 occurrences are required")
	}
	if len(recurring.Notes) > 2000 {
		return fmt.Errorf("notes cannot exceed 2000 characters")
	}
	switch recurring.Interval {
	case "daily", "weekly", "monthly", "yearly":
		return nil
	default:
		return fmt.Errorf("interval must be daily, weekly, monthly, or yearly")
	}
}

func normalizeOwner(owner string) string {
	owner = strings.ToLower(SanitizeString(owner))
	if owner == "" {
		return "common"
	}
	return owner
}
