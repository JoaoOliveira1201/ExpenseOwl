package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tanq16/expenseowl/internal/storage"
	"github.com/tanq16/expenseowl/internal/web"
)

type Handler struct {
	storage    storage.Store
	receiptDir string
}

func NewHandler(store storage.Store) *Handler {
	receiptDir := os.Getenv("RECEIPT_DIR")
	if receiptDir == "" {
		receiptDir = filepath.Join("data", "receipts")
	}
	if err := os.MkdirAll(receiptDir, 0o755); err != nil {
		log.Printf("create receipt directory: %v", err)
	}
	return &Handler{storage: store, receiptDir: receiptDir}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func method(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Method == expected {
		return true
	}
	w.Header().Set("Allow", expected)
	writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"Method not allowed"})
	return false
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	config, err := h.storage.GetConfig(r.Context())
	if err != nil {
		h.serverError(w, "load configuration", err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	config, err := h.storage.GetConfig(r.Context())
	if err != nil {
		h.serverError(w, "load categories", err)
		return
	}
	writeJSON(w, http.StatusOK, config.Categories)
}

func (h *Handler) UpdateCategories(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	var input []string
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	seen := map[string]bool{}
	categories := make([]string, 0, len(input))
	for _, candidate := range input {
		category, err := storage.ValidateCategory(candidate)
		key := strings.ToLower(category)
		if err != nil || seen[key] {
			continue
		}
		seen[key] = true
		categories = append(categories, category)
	}
	if len(categories) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"Keep at least one category"})
		return
	}
	if err := h.storage.UpdateCategories(r.Context(), categories); err != nil {
		h.serverError(w, "save categories", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetCategoryTargets(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	config, err := h.storage.GetConfig(r.Context())
	if err != nil {
		h.serverError(w, "load category targets", err)
		return
	}
	writeJSON(w, http.StatusOK, config.CategoryTargets)
}

func (h *Handler) UpdateCategoryTargets(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	var targets map[string]float64
	if err := decodeJSON(r, &targets); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	config, err := h.storage.GetConfig(r.Context())
	if err != nil {
		h.serverError(w, "validate targets", err)
		return
	}
	valid := map[string]bool{}
	for _, category := range config.Categories {
		valid[category] = true
	}
	clean := map[string]float64{}
	for category, amount := range targets {
		if !valid[category] {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"Unknown category: " + category})
			return
		}
		if amount < 0 || amount > 9e15 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"Invalid target for " + category})
			return
		}
		if amount > 0 {
			clean[category] = amount
		}
	}
	if err := h.storage.UpdateCategoryTargets(r.Context(), clean); err != nil {
		h.serverError(w, "save category targets", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) UpdateCategoryParents(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	var parents map[string]string
	if err := decodeJSON(r, &parents); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	config, err := h.storage.GetConfig(r.Context())
	if err != nil {
		h.serverError(w, "validate category parents", err)
		return
	}
	clean := make(map[string]string, len(config.Categories))
	for _, category := range config.Categories {
		parent := parents[category]
		if !storage.ValidateCategoryParent(parent) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"Choose essentials or lifestyle for " + category})
			return
		}
		clean[category] = parent
	}
	if err := h.storage.UpdateCategoryParents(r.Context(), clean); err != nil {
		h.serverError(w, "save category parents", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetExpenses(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	filter, err := expenseFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	expenses, err := h.storage.GetExpenses(r.Context(), filter)
	if err != nil {
		h.serverError(w, "load transactions", err)
		return
	}
	writeJSON(w, http.StatusOK, expenses)
}

func (h *Handler) AddExpense(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	var expense storage.Expense
	if err := decodeJSON(r, &expense); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	if err := expense.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	created, err := h.storage.AddExpense(r.Context(), expense)
	if err != nil {
		h.serverError(w, "save transaction", err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) EditExpense(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"ID is required"})
		return
	}
	var expense storage.Expense
	if err := decodeJSON(r, &expense); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	if err := expense.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	previous, _ := h.storage.GetExpense(r.Context(), id)
	updated, err := h.storage.UpdateExpense(r.Context(), id, expense)
	if err != nil {
		h.serverError(w, "update transaction", err)
		return
	}
	if previous.Receipt != "" && previous.Receipt != updated.Receipt {
		h.removeReceipt(previous.Receipt)
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodDelete) {
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"ID is required"})
		return
	}
	expense, _ := h.storage.GetExpense(r.Context(), id)
	if err := h.storage.RemoveExpense(r.Context(), id); err != nil {
		h.serverError(w, "delete transaction", err)
		return
	}
	h.removeReceipt(expense.Receipt)
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) DeleteMultipleExpenses(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodDelete) {
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	for _, id := range input.IDs {
		if expense, err := h.storage.GetExpense(r.Context(), id); err == nil {
			defer h.removeReceipt(expense.Receipt)
		}
	}
	if err := h.storage.RemoveExpenses(r.Context(), input.IDs); err != nil {
		h.serverError(w, "delete transactions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	items, err := h.storage.GetRecurringExpenses(r.Context())
	if err != nil {
		h.serverError(w, "load recurring transactions", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) AddRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	var recurring storage.RecurringExpense
	if err := decodeJSON(r, &recurring); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	if err := recurring.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	created, err := h.storage.AddRecurringExpense(r.Context(), recurring)
	if err != nil {
		h.serverError(w, "save recurring transaction", err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) UpdateRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPut) {
		return
	}
	id := r.URL.Query().Get("id")
	updateAll, _ := strconv.ParseBool(r.URL.Query().Get("updateAll"))
	var recurring storage.RecurringExpense
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"ID is required"})
		return
	}
	if err := decodeJSON(r, &recurring); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	if err := recurring.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
		return
	}
	if err := h.storage.UpdateRecurringExpense(r.Context(), id, recurring, updateAll); err != nil {
		h.serverError(w, "update recurring transaction", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) DeleteRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodDelete) {
		return
	}
	id := r.URL.Query().Get("id")
	removeAll, _ := strconv.ParseBool(r.URL.Query().Get("removeAll"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"ID is required"})
		return
	}
	if err := h.storage.RemoveRecurringExpense(r.Context(), id, removeAll); err != nil {
		h.serverError(w, "delete recurring transaction", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) UploadReceipt(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	file, _, err := r.FormFile("receipt")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"Choose a receipt image or PDF up to 5 MB"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 5<<20 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"Receipt must be between 1 byte and 5 MB"})
		return
	}
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "application/pdf": ".pdf"}
	extension, ok := extensions[http.DetectContentType(data)]
	if !ok {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"Receipt must be a JPG, PNG, WebP, or PDF"})
		return
	}
	name := uuid.NewString() + extension
	if err := os.MkdirAll(h.receiptDir, 0o755); err != nil {
		h.serverError(w, "prepare receipt storage", err)
		return
	}
	if err := os.WriteFile(filepath.Join(h.receiptDir, name), data, 0o644); err != nil {
		h.serverError(w, "store receipt", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"receipt": "/receipts/" + name})
}

func (h *Handler) ServeReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"Method not allowed"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/receipts/")
	if name == "" || name != filepath.Base(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, filepath.Join(h.receiptDir, name))
}

func (h *Handler) ServePage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, http.MethodGet) {
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := web.ServeTemplate(w, name); err != nil {
			http.Error(w, "Page unavailable", http.StatusInternalServerError)
		}
	}
}

func (h *Handler) ServeStaticFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"Method not allowed"})
		return
	}
	path := r.URL.Path
	if path == "/manifest.json" {
		path = "/manifest.webmanifest"
	}
	if path == "/sw.js" || path == "/manifest.webmanifest" {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if path == "/sw.js" {
		w.Header().Set("Service-Worker-Allowed", "/")
	}
	if err := web.ServeStatic(w, path); err != nil {
		http.NotFound(w, r)
	}
}

func (h *Handler) removeReceipt(reference string) {
	name := strings.TrimPrefix(reference, "/receipts/")
	if name == "" || name != filepath.Base(name) {
		return
	}
	if err := os.Remove(filepath.Join(h.receiptDir, name)); err != nil && !os.IsNotExist(err) {
		log.Printf("remove receipt %s: %v", name, err)
	}
}

func (h *Handler) serverError(w http.ResponseWriter, action string, err error) {
	log.Printf("%s: %v", action, err)
	writeJSON(w, http.StatusInternalServerError, ErrorResponse{"Could not " + action})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("Invalid request: %v", err)
	}
	return nil
}

func expenseFilter(r *http.Request) (storage.ExpenseFilter, error) {
	filter := storage.ExpenseFilter{Owner: r.URL.Query().Get("owner")}
	for value, destination := range map[string]**time.Time{"from": &filter.From, "to": &filter.To} {
		raw := r.URL.Query().Get(value)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("%s must be an RFC3339 timestamp", value)
		}
		*destination = &parsed
	}
	return filter, nil
}
