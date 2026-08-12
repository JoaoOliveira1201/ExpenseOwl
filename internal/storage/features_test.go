package storage

import (
	"testing"
	"time"
)

func TestJSONStorePersistsTargetsNotesAndReceipt(t *testing.T) {
	dir := t.TempDir()
	store, err := InitializeJsonStore(SystemConfig{StorageURL: dir})
	if err != nil {
		t.Fatal(err)
	}

	targets := map[string]float64{"Food": 350, "Travel": 125.50}
	if err := store.UpdateCategoryTargets(targets); err != nil {
		t.Fatal(err)
	}
	gotTargets, err := store.GetCategoryTargets()
	if err != nil {
		t.Fatal(err)
	}
	if gotTargets["Food"] != 350 || gotTargets["Travel"] != 125.50 {
		t.Fatalf("unexpected targets: %#v", gotTargets)
	}

	expense := Expense{
		Name:     "Groceries",
		Category: "Food",
		Amount:   -42.25,
		Date:     time.Now(),
		Notes:    "Weekly shop\nUsed a coupon",
		Receipt:  "/receipts/example.png",
	}
	if err := expense.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := store.AddExpense(expense); err != nil {
		t.Fatal(err)
	}
	expenses, err := store.GetAllExpenses()
	if err != nil {
		t.Fatal(err)
	}
	if len(expenses) != 1 || expenses[0].Notes != expense.Notes || expenses[0].Receipt != expense.Receipt {
		t.Fatalf("expense metadata was not preserved: %#v", expenses)
	}
}

func TestExpenseRejectsUnsafeReceiptReference(t *testing.T) {
	expense := Expense{
		Name:     "Test",
		Category: "Food",
		Amount:   -1,
		Date:     time.Now(),
		Receipt:  "/receipts/../secret",
	}
	if err := expense.Validate(); err == nil {
		t.Fatal("expected unsafe receipt reference to be rejected")
	}
}
