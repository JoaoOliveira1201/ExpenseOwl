package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tanq16/expenseowl/internal/storage"
)

func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	expenses, err := h.storage.GetExpenses(r.Context(), storage.ExpenseFilter{})
	if err != nil {
		h.serverError(w, "export transactions", err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="expenseowl.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"ID", "Name", "Category", "Amount", "Date", "Owner", "Notes", "Receipt"})
	for _, expense := range expenses {
		_ = writer.Write([]string{expense.ID, expense.Name, expense.Category, strconv.FormatFloat(expense.Amount, 'f', 2, 64), expense.Date.Format(time.RFC3339), expense.Owner, expense.Notes, expense.Receipt})
	}
	writer.Flush()
}

type importResult struct {
	Status         string   `json:"status"`
	TotalProcessed int      `json:"total_processed"`
	Imported       int      `json:"imported"`
	Skipped        int      `json:"skipped"`
	NewCategories  []string `json:"new_categories"`
}

func (h *Handler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"Attach a CSV file in the file field"})
		return
	}
	defer file.Close()

	config, err := h.storage.GetConfig(r.Context())
	if err != nil {
		h.serverError(w, "load categories for import", err)
		return
	}
	expenses, categories, total, skipped, err := parseCSV(file, config.Categories)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	inserted, err := h.storage.AddExpenses(r.Context(), expenses)
	if err != nil {
		h.serverError(w, "import transactions", err)
		return
	}
	if len(categories) > len(config.Categories) {
		if err := h.storage.UpdateCategories(r.Context(), categories); err != nil {
			h.serverError(w, "save imported categories", err)
			return
		}
	}
	newCategories := categories[len(config.Categories):]
	writeJSON(w, http.StatusOK, importResult{Status: "success", TotalProcessed: total, Imported: inserted, Skipped: skipped + len(expenses) - inserted, NewCategories: newCategories})
}

func parseCSV(source io.Reader, existingCategories []string) ([]storage.Expense, []string, int, int, error) {
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("CSV is empty or unreadable")
	}
	columns := map[string]int{}
	for index, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = index
	}
	for _, required := range []string{"name", "category", "amount", "date"} {
		if _, ok := columns[required]; !ok {
			return nil, nil, 0, 0, fmt.Errorf("Missing required column: %s", required)
		}
	}
	value := func(row []string, name string) string {
		index, ok := columns[name]
		if !ok || index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}
	categories := append([]string(nil), existingCategories...)
	categorySet := map[string]bool{}
	for _, category := range categories {
		categorySet[strings.ToLower(category)] = true
	}
	expenses := make([]storage.Expense, 0)
	total, skipped := 0, 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		total++
		if err != nil {
			skipped++
			continue
		}
		amount, amountErr := strconv.ParseFloat(value(row, "amount"), 64)
		date, dateErr := parseDate(value(row, "date"))
		category := value(row, "category")
		expense := storage.Expense{ID: value(row, "id"), Name: value(row, "name"), Category: category, Amount: amount, Date: date, Owner: value(row, "owner"), Notes: value(row, "notes"), Receipt: value(row, "receipt")}
		if amountErr != nil || dateErr != nil || expense.Validate() != nil {
			skipped++
			continue
		}
		key := strings.ToLower(expense.Category)
		if !categorySet[key] {
			categories = append(categories, expense.Category)
			categorySet[key] = true
		}
		expenses = append(expenses, expense)
	}
	return expenses, categories, total, skipped, nil
}

func parseDate(value string) (time.Time, error) {
	for _, format := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02", "2006-1-2", "2006/01/02", "2006/1/2"} {
		if date, err := time.Parse(format, value); err == nil {
			return date.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date")
}
