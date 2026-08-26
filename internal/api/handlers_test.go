package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tanq16/expenseowl/internal/storage"
)

func TestExpenseFilterDefaultsUnboundedRequestToOnePage(t *testing.T) {
	filter, err := expenseFilter(httptest.NewRequest("GET", "/expenses", nil))
	if err != nil {
		t.Fatal(err)
	}
	if filter.Limit != 100 {
		t.Fatalf("expected default limit 100, got %d", filter.Limit)
	}
}

func TestExpenseFilterLeavesDateRangeUnpaginated(t *testing.T) {
	request := httptest.NewRequest("GET", "/expenses?from=2026-08-01T00:00:00Z&to=2026-09-01T00:00:00Z", nil)
	filter, err := expenseFilter(request)
	if err != nil {
		t.Fatal(err)
	}
	if filter.Limit != 0 {
		t.Fatalf("expected date range without a limit to remain unpaginated, got %d", filter.Limit)
	}
}

func TestExpenseCursorRoundTrip(t *testing.T) {
	expense := storage.Expense{ID: "expense-id", Date: time.Date(2026, 8, 26, 14, 30, 0, 123, time.FixedZone("test", 2*60*60))}
	request := httptest.NewRequest("GET", "/expenses?limit=25&cursor="+encodeExpenseCursor(expense), nil)
	filter, err := expenseFilter(request)
	if err != nil {
		t.Fatal(err)
	}
	if filter.Limit != 25 || filter.Cursor == nil || filter.Cursor.ID != expense.ID || !filter.Cursor.Date.Equal(expense.Date) {
		t.Fatalf("unexpected pagination filter: %#v", filter)
	}
}

func TestExpenseFilterRejectsInvalidPagination(t *testing.T) {
	for _, target := range []string{"/expenses?limit=0", "/expenses?limit=201", "/expenses?cursor=invalid"} {
		if _, err := expenseFilter(httptest.NewRequest("GET", target, nil)); err == nil {
			t.Fatalf("expected %s to be rejected", target)
		}
	}
}
