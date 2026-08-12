package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	Currency        = "EUR"
	StartDate       = 1
	ParentEssential = "essentials"
	ParentLifestyle = "lifestyle"
)

var defaultCategories = []string{
	"Food", "Groceries", "Travel", "Rent", "Utilities",
	"Entertainment", "Healthcare", "Shopping", "Miscellaneous",
}

// Store is the persistence contract used by the HTTP layer. ExpenseOwl has one
// supported backend: PostgreSQL.
type Store interface {
	Close() error
	GetConfig(context.Context) (Config, error)
	UpdateCategories(context.Context, []string) error
	UpdateCategoryTargets(context.Context, map[string]float64) error
	UpdateCategoryParents(context.Context, map[string]string) error
	UpdateAllocationTargets(context.Context, AllocationTargets) error

	GetExpenses(context.Context, ExpenseFilter) ([]Expense, error)
	GetExpense(context.Context, string) (Expense, error)
	AddExpense(context.Context, Expense) (Expense, error)
	UpdateExpense(context.Context, string, Expense) (Expense, error)
	RemoveExpense(context.Context, string) error
	RemoveExpenses(context.Context, []string) error

	GetRecurringExpenses(context.Context) ([]RecurringExpense, error)
	AddRecurringExpense(context.Context, RecurringExpense) (RecurringExpense, error)
	UpdateRecurringExpense(context.Context, string, RecurringExpense, bool) error
	RemoveRecurringExpense(context.Context, string, bool) error
}

type Config struct {
	Categories        []string           `json:"categories"`
	CategoryTargets   map[string]float64 `json:"categoryTargets"`
	CategoryParents   map[string]string  `json:"categoryParents"`
	AllocationTargets AllocationTargets `json:"allocationTargets"`
	Currency          string             `json:"currency"`
	StartDate         int                `json:"startDate"`
}

type AllocationTargets struct {
	EssentialsMax float64 `json:"essentialsMax"`
	LifestyleMax  float64 `json:"lifestyleMax"`
	SavingsMin    float64 `json:"savingsMin"`
}

func DefaultAllocationTargets() AllocationTargets {
	return AllocationTargets{EssentialsMax: 50, LifestyleMax: 30, SavingsMin: 20}
}

func (targets AllocationTargets) Validate() error {
	values := map[string]float64{
		"essentials maximum": targets.EssentialsMax,
		"lifestyle maximum":  targets.LifestyleMax,
		"savings minimum":    targets.SavingsMin,
	}
	for label, value := range values {
		if value < 0 || value > 100 {
			return fmt.Errorf("%s must be between 0%% and 100%%", label)
		}
	}
	return nil
}

func DefaultCategoryParent(category string) string {
	switch strings.ToLower(category) {
	case "entertainment", "shopping", "miscellaneous":
		return ParentLifestyle
	default:
		return ParentEssential
	}
}

func ValidateCategoryParent(parent string) bool {
	return parent == ParentEssential || parent == ParentLifestyle
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
	if strings.EqualFold(category, "income") {
		return "", fmt.Errorf("income is a transaction type, not a category")
	}
	return category, nil
}

func (expense *Expense) Validate() error {
	expense.Name = SanitizeString(expense.Name)
	expense.Category = SanitizeString(expense.Category)
	expense.Owner = normalizeOwner(expense.Owner)
	expense.Notes = SanitizeNotes(expense.Notes)
	if expense.Name == "" {
		return fmt.Errorf("name is required")
	}
	if expense.Amount == 0 {
		return fmt.Errorf("amount cannot be zero")
	}
	if expense.Amount > 0 {
		expense.Category = ""
	} else if _, err := ValidateCategory(expense.Category); err != nil {
		return err
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
	if recurring.Name == "" {
		return fmt.Errorf("name is required")
	}
	if recurring.Amount == 0 {
		return fmt.Errorf("amount cannot be zero")
	}
	if recurring.Amount > 0 {
		recurring.Category = ""
	} else if _, err := ValidateCategory(recurring.Category); err != nil {
		return err
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
