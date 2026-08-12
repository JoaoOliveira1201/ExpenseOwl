package storage

import (
	"testing"
	"time"
)

func TestExpenseValidationAndFixedConventions(t *testing.T) {
	expense := Expense{Name: " Groceries ", Category: "Food", Amount: -42.25, Date: time.Now(), Notes: "Weekly shop\nUsed a coupon", Receipt: "/receipts/example.png"}
	if err := expense.Validate(); err != nil {
		t.Fatal(err)
	}
	if expense.Owner != "common" {
		t.Fatalf("expected common owner, got %q", expense.Owner)
	}
	if Currency != "EUR" || StartDate != 1 {
		t.Fatal("fixed conventions changed")
	}
}

func TestExpenseRejectsUnsafeReceiptReference(t *testing.T) {
	expense := Expense{Name: "Test", Category: "Food", Amount: -1, Date: time.Now(), Receipt: "/receipts/../secret"}
	if err := expense.Validate(); err == nil {
		t.Fatal("expected unsafe receipt reference to be rejected")
	}
}

func TestGenerateMonthlyExpenses(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	items := generateExpenses(RecurringExpense{ID: "rule", Name: "Rent", Category: "Rent", Amount: -900, StartDate: start, Interval: "monthly", Occurrences: 3, Owner: "common"}, false)
	if len(items) != 3 || !items[2].Date.Equal(start.AddDate(0, 2, 0)) {
		t.Fatalf("unexpected generated expenses: %#v", items)
	}
}
