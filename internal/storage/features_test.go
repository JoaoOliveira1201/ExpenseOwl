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

func TestIncomeDoesNotHaveCategory(t *testing.T) {
	income := Expense{Name: "Salary", Category: "Food", Amount: 2500, Date: time.Now()}
	if err := income.Validate(); err != nil {
		t.Fatal(err)
	}
	if income.Category != "" {
		t.Fatalf("expected income category to be cleared, got %q", income.Category)
	}
}

func TestSpendingRequiresNonIncomeCategory(t *testing.T) {
	for _, category := range []string{"", "Income", "income"} {
		expense := Expense{Name: "Test", Category: category, Amount: -1, Date: time.Now()}
		if err := expense.Validate(); err == nil {
			t.Fatalf("expected category %q to be rejected", category)
		}
	}
}

func TestRecurringIncomeDoesNotHaveCategory(t *testing.T) {
	income := RecurringExpense{Name: "Salary", Category: "Income", Amount: 2500, StartDate: time.Now(), Interval: "monthly", Occurrences: 2}
	if err := income.Validate(); err != nil {
		t.Fatal(err)
	}
	if income.Category != "" {
		t.Fatalf("expected recurring income category to be cleared, got %q", income.Category)
	}
}

func TestCategoryParentDefaultsAndValidation(t *testing.T) {
	if DefaultCategoryParent("Shopping") != ParentLifestyle {
		t.Fatal("shopping should default to lifestyle")
	}
	if DefaultCategoryParent("Rent") != ParentEssential {
		t.Fatal("rent should default to essentials")
	}
	if !ValidateCategoryParent(ParentEssential) || !ValidateCategoryParent(ParentLifestyle) || ValidateCategoryParent("other") {
		t.Fatal("category parent validation accepted an unexpected value")
	}
}

func TestGenerateMonthlyExpenses(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	items := generateExpenses(RecurringExpense{ID: "rule", Name: "Rent", Category: "Rent", Amount: -900, StartDate: start, Interval: "monthly", Occurrences: 3, Owner: "common"}, false)
	if len(items) != 3 || !items[2].Date.Equal(start.AddDate(0, 2, 0)) {
		t.Fatalf("unexpected generated expenses: %#v", items)
	}
}
