package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tanq16/expenseowl/internal/storage"
)

const mcpTestToken = "test-only-mcp-token"

type mcpTestStore struct {
	storage.Store
	expense   storage.Expense
	recurring storage.RecurringExpense
	filter    storage.ExpenseFilter
	config    storage.Config
	writes    int
	updateAll bool
	removeAll bool
}

func (s *mcpTestStore) GetConfig(context.Context) (storage.Config, error) { return s.config, nil }
func (s *mcpTestStore) GetExpense(_ context.Context, id string) (storage.Expense, error) {
	if s.expense.ID != id || id == "" {
		return storage.Expense{}, fmt.Errorf("expense not found")
	}
	return s.expense, nil
}
func (s *mcpTestStore) GetExpenses(_ context.Context, filter storage.ExpenseFilter) ([]storage.Expense, error) {
	s.filter = filter
	if filter.Cursor != nil {
		return []storage.Expense{{ID: "last", Date: s.expense.Date.Add(-time.Hour)}}, nil
	}
	return []storage.Expense{s.expense, {ID: "second", Date: s.expense.Date}, {ID: "last", Date: s.expense.Date.Add(-time.Hour)}}, nil
}
func (s *mcpTestStore) AddExpense(_ context.Context, expense storage.Expense) (storage.Expense, error) {
	expense.ID = "created"
	s.expense = expense
	s.writes++
	return expense, nil
}
func (s *mcpTestStore) UpdateExpense(_ context.Context, id string, expense storage.Expense) (storage.Expense, error) {
	expense.ID = id
	s.expense = expense
	s.writes++
	return expense, nil
}
func (s *mcpTestStore) RemoveExpense(_ context.Context, id string) error {
	if id != s.expense.ID {
		return fmt.Errorf("expense not found")
	}
	s.expense = storage.Expense{}
	s.writes++
	return nil
}
func (s *mcpTestStore) GetRecurringExpenses(context.Context) ([]storage.RecurringExpense, error) {
	return []storage.RecurringExpense{s.recurring}, nil
}
func (s *mcpTestStore) AddRecurringExpense(_ context.Context, item storage.RecurringExpense) (storage.RecurringExpense, error) {
	item.ID = "recurring-created"
	s.recurring = item
	s.writes++
	return item, nil
}
func (s *mcpTestStore) UpdateRecurringExpense(_ context.Context, id string, item storage.RecurringExpense, all bool) error {
	item.ID = id
	s.recurring, s.updateAll = item, all
	s.writes++
	return nil
}
func (s *mcpTestStore) RemoveRecurringExpense(_ context.Context, _ string, all bool) error {
	s.recurring, s.removeAll = storage.RecurringExpense{}, all
	s.writes++
	return nil
}
func (s *mcpTestStore) UpdateCategories(_ context.Context, categories []string) error {
	s.config.Categories = categories
	return nil
}
func (s *mcpTestStore) UpdateCategoryTargets(_ context.Context, targets map[string]float64) error {
	s.config.CategoryTargets = targets
	return nil
}
func (s *mcpTestStore) UpdateCategoryParents(_ context.Context, parents map[string]string) error {
	s.config.CategoryParents = parents
	return nil
}
func (s *mcpTestStore) UpdateAllocationTargets(_ context.Context, targets storage.AllocationTargets) error {
	s.config.AllocationTargets = targets
	return nil
}

type mcpTestTransport struct{ base http.RoundTripper }

func (transport mcpTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+mcpTestToken)
	return transport.base.RoundTrip(request)
}

func mcpTestSession(t *testing.T, handler *Handler) (*mcp.ClientSession, context.Context) {
	t.Helper()
	server := httptest.NewServer(handler.MCPHandler(mcpTestToken, "test"))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	httpClient := server.Client()
	httpClient.Transport = mcpTestTransport{base: httpClient.Transport}
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL, HTTPClient: httpClient}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

func callMCP(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, input map[string]any) map[string]any {
	t.Helper()
	if input == nil {
		input = map[string]any{}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("%s failed: %+v", name, result.Content)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	return output
}

func TestMCPAuthentication(t *testing.T) {
	for _, test := range []struct {
		name, token, authorization, origin string
		status                             int
	}{
		{name: "disabled", status: http.StatusNotFound},
		{name: "blank token", token: "  ", status: http.StatusNotFound},
		{name: "missing token", token: mcpTestToken, status: http.StatusUnauthorized},
		{name: "wrong token", token: mcpTestToken, authorization: "Bearer wrong", status: http.StatusUnauthorized},
		{name: "browser origin", token: mcpTestToken, authorization: "Bearer " + mcpTestToken, origin: "https://example.com", status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := &Handler{storage: &mcpTestStore{}}
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			request.Header.Set("Authorization", test.authorization)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			handler.MCPHandler(test.token, "test").ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("got HTTP %d, want %d", response.Code, test.status)
			}
			if test.status == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing bearer authentication challenge")
			}
		})
	}
}

func TestMCPDiscoveryAndPagination(t *testing.T) {
	store := &mcpTestStore{expense: storage.Expense{ID: "first", Date: time.Now()}, config: storage.Config{Currency: "EUR", Categories: []string{"Food"}}}
	session, ctx := mcpTestSession(t, &Handler{storage: store})
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 14 {
		t.Fatalf("expected 14 tools, got %d", len(tools.Tools))
	}
	readOnly := map[string]bool{"get_config": true, "get_expense": true, "list_expenses": true, "list_recurring_expenses": true}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != readOnly[tool.Name] {
			t.Fatalf("incorrect read/write annotation on %s", tool.Name)
		}
	}
	if config := callMCP(t, ctx, session, "get_config", nil); config["currency"] != "EUR" {
		t.Fatalf("wrong config: %+v", config)
	}
	callMCP(t, ctx, session, "list_expenses", map[string]any{"from": "2026-10-01T00:00:00Z", "owner": "common"})
	if store.filter.Limit != 100 || store.filter.From == nil || store.filter.Owner != "common" {
		t.Fatalf("date-filtered MCP requests must remain bounded: %+v", store.filter)
	}
	page := callMCP(t, ctx, session, "list_expenses", map[string]any{"limit": 2})
	if len(page["expenses"].([]any)) != 2 || page["nextCursor"] == nil {
		t.Fatalf("invalid first page: %+v", page)
	}
	page = callMCP(t, ctx, session, "list_expenses", map[string]any{"limit": 2, "cursor": page["nextCursor"]})
	if store.filter.Cursor == nil || store.filter.Cursor.ID != "second" || len(page["expenses"].([]any)) != 1 || page["nextCursor"] != nil {
		t.Fatalf("invalid final page: %+v", page)
	}
}

func TestMCPTransactionMutationsPreserveReceipts(t *testing.T) {
	store := &mcpTestStore{}
	handler := &Handler{storage: store, receiptDir: t.TempDir()}
	session, ctx := mcpTestSession(t, handler)
	created := callMCP(t, ctx, session, "add_expense", map[string]any{"name": "Lunch", "amount": -12.50, "category": "Food", "date": "2026-10-02T12:00:00+01:00"})
	if created["id"] != "created" || store.expense.Owner != "common" || store.expense.Amount != -12.50 {
		t.Fatalf("invalid created transaction: %+v", created)
	}
	store.expense.Receipt, store.expense.RecurringID = "/receipts/test.png", "recurring-id"
	receiptPath := filepath.Join(handler.receiptDir, "test.png")
	if err := os.WriteFile(receiptPath, []byte("receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	callMCP(t, ctx, session, "update_expense", map[string]any{"id": "created", "notes": "Paid in cash"})
	if store.expense.Notes != "Paid in cash" || store.expense.Receipt != "/receipts/test.png" || store.expense.RecurringID != "recurring-id" || store.expense.Name != "Lunch" || store.expense.Amount != -12.50 {
		t.Fatalf("partial update lost existing fields: %+v", store.expense)
	}
	if _, err := os.Stat(receiptPath); err != nil {
		t.Fatalf("partial update deleted receipt: %v", err)
	}
	if expense := callMCP(t, ctx, session, "get_expense", map[string]any{"id": "created"}); expense["receipt"] != "/receipts/test.png" {
		t.Fatalf("receipt reference missing: %+v", expense)
	}
	callMCP(t, ctx, session, "delete_expense", map[string]any{"id": "created"})
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) || store.expense.ID != "" {
		t.Fatalf("delete did not remove transaction and receipt: %v", err)
	}
	callMCP(t, ctx, session, "add_expense", map[string]any{"name": "Salary", "amount": 2000, "category": "Food", "date": "2026-10-02T12:00:00Z"})
	if store.expense.Category != "" {
		t.Fatal("income should not retain a category")
	}
}

func TestMCPRejectsInvalidWrites(t *testing.T) {
	store := &mcpTestStore{}
	session, ctx := mcpTestSession(t, &Handler{storage: store})
	for _, input := range []map[string]any{
		{"name": "Lunch", "amount": -12, "date": "2026-10-02T12:00:00Z"},
		{"name": "Lunch", "amount": 0, "category": "Food", "date": "2026-10-02T12:00:00Z"},
		{"name": "Lunch", "amount": -12, "category": "Food", "date": "bad date"},
		{"name": "Lunch", "amount": -12, "category": "Food", "date": "2026-10-02T12:00:00Z", "id": "client-supplied"},
		{"amount": -12, "category": "Food", "date": "2026-10-02T12:00:00Z"},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "add_expense", Arguments: input})
		if err == nil && !result.IsError {
			t.Fatalf("invalid write accepted: %+v", input)
		}
	}
	if store.writes != 0 {
		t.Fatalf("invalid writes reached storage: %d", store.writes)
	}
}

func TestMCPRecurringAndSettings(t *testing.T) {
	store := &mcpTestStore{config: storage.Config{Categories: []string{"Food"}}}
	session, ctx := mcpTestSession(t, &Handler{storage: store})
	callMCP(t, ctx, session, "add_recurring_expense", map[string]any{"name": "Groceries", "amount": -100, "category": "Food", "startDate": "2026-10-02T12:00:00Z", "interval": "monthly", "occurrences": 12})
	page := callMCP(t, ctx, session, "list_recurring_expenses", nil)
	if len(page["recurringExpenses"].([]any)) != 1 {
		t.Fatalf("invalid recurring list: %+v", page)
	}
	callMCP(t, ctx, session, "update_recurring_expense", map[string]any{"id": "recurring-created", "amount": -120})
	if store.recurring.Amount != -120 || store.recurring.Name != "Groceries" || store.recurring.Occurrences != 12 || store.updateAll {
		t.Fatalf("partial recurring update failed: %+v", store.recurring)
	}
	callMCP(t, ctx, session, "update_recurring_expense", map[string]any{"id": "recurring-created", "notes": "All entries", "updateAll": true})
	if !store.updateAll || store.recurring.Notes != "All entries" {
		t.Fatal("updateAll was not forwarded")
	}
	callMCP(t, ctx, session, "delete_recurring_expense", map[string]any{"id": "recurring-created", "removeAll": true})
	if !store.removeAll {
		t.Fatal("removeAll was not forwarded")
	}
	callMCP(t, ctx, session, "update_categories", map[string]any{"categories": []string{"Food", "Food", "Rent"}})
	if len(store.config.Categories) != 2 {
		t.Fatal("category validation was bypassed")
	}
	callMCP(t, ctx, session, "update_category_targets", map[string]any{"targets": map[string]float64{"Food": 300}})
	callMCP(t, ctx, session, "update_category_parents", map[string]any{"parents": map[string]string{"Food": "lifestyle", "Rent": "essentials"}})
	callMCP(t, ctx, session, "update_allocation_targets", map[string]any{"essentialsMax": 60, "lifestyleMax": 25, "savingsMin": 15})
	if store.config.CategoryTargets["Food"] != 300 || store.config.CategoryParents["Food"] != "lifestyle" || store.config.AllocationTargets.SavingsMin != 15 {
		t.Fatalf("settings not saved: %+v", store.config)
	}
}
