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

// Handler holds the storage interface
type Handler struct {
	storage    storage.Storage
	receiptDir string
}

// NewHandler creates a new API handler
func NewHandler(s storage.Storage) *Handler {
	receiptDir := os.Getenv("RECEIPT_DIR")
	if receiptDir == "" {
		receiptDir = filepath.Join("data", "receipts")
	}
	if err := os.MkdirAll(receiptDir, 0755); err != nil {
		log.Printf("WARNING: Failed to create receipt directory: %v", err)
	}
	return &Handler{
		storage:    s,
		receiptDir: receiptDir,
	}
}

// ErrorResponse is a generic JSON error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeJSON is a helper to write JSON responses
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

// ------------------------------------------------------------
// Config Handlers
// ------------------------------------------------------------

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	config, err := h.storage.GetConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get config"})
		log.Printf("API ERROR: Failed to get config: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	categories, err := h.storage.GetCategories()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get categories"})
		log.Printf("API ERROR: Failed to get categories: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

func (h *Handler) UpdateCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var categories []string
	if err := json.NewDecoder(r.Body).Decode(&categories); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	var sanitizedCategories []string
	for _, category := range categories {
		sanitized, err := storage.ValidateCategory(category)
		if err != nil {
			log.Printf("API ERROR: Invalid category provided: %v\n", err)
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Invalid category '%s': %v", category, err)})
			return
		}
		sanitizedCategories = append(sanitizedCategories, sanitized)
	}
	if err := h.storage.UpdateCategories(sanitizedCategories); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update categories"})
		log.Printf("API ERROR: Failed to update categories: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetCurrency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	currency, err := h.storage.GetCurrency()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get currency"})
		log.Printf("API ERROR: Failed to get currency: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, currency)
}

func (h *Handler) UpdateCurrency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var currency string
	if err := json.NewDecoder(r.Body).Decode(&currency); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := h.storage.UpdateCurrency(currency); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		log.Printf("API ERROR: Failed to update currency: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetStartDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	startDate, err := h.storage.GetStartDate()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get start date"})
		log.Printf("API ERROR: Failed to get start date: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, startDate)
}

func (h *Handler) UpdateStartDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var startDate int
	if err := json.NewDecoder(r.Body).Decode(&startDate); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := h.storage.UpdateStartDate(startDate); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		log.Printf("API ERROR: Failed to update start date: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetCategoryTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	targets, err := h.storage.GetCategoryTargets()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get category targets"})
		return
	}
	writeJSON(w, http.StatusOK, targets)
}

func (h *Handler) UpdateCategoryTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var targets map[string]float64
	if err := json.NewDecoder(r.Body).Decode(&targets); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	categories, err := h.storage.GetCategories()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to validate category targets"})
		return
	}
	validCategories := make(map[string]bool, len(categories))
	for _, category := range categories {
		validCategories[category] = true
	}
	cleanTargets := make(map[string]float64)
	for category, amount := range targets {
		if !validCategories[category] {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown category: %s", category)})
			return
		}
		if amount < 0 || amount > 9000000000000000 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Invalid target for %s", category)})
			return
		}
		if amount > 0 {
			cleanTargets[category] = amount
		}
	}
	if err := h.storage.UpdateCategoryTargets(cleanTargets); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update category targets"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// ------------------------------------------------------------
// Expense Handlers
// ------------------------------------------------------------

func (h *Handler) AddExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var expense storage.Expense
	if err := json.NewDecoder(r.Body).Decode(&expense); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := expense.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if expense.Date.IsZero() {
		expense.Date = time.Now()
	}
	if err := h.storage.AddExpense(expense); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to save expense"})
		log.Printf("API ERROR: Failed to save expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, expense)
}

func (h *Handler) GetExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	expenses, err := h.storage.GetAllExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve expenses"})
		log.Printf("API ERROR: Failed to retrieve expenses: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, expenses)
}

func (h *Handler) EditExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	var expense storage.Expense
	if err := json.NewDecoder(r.Body).Decode(&expense); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := expense.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	previous, _ := h.storage.GetExpense(id)
	if err := h.storage.UpdateExpense(id, expense); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to edit expense"})
		log.Printf("API ERROR: Failed to edit expense: %v\n", err)
		return
	}
	if previous.Receipt != "" && previous.Receipt != expense.Receipt {
		h.removeReceipt(previous.Receipt)
	}
	writeJSON(w, http.StatusOK, expense)
}

func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	expense, _ := h.storage.GetExpense(id)
	if err := h.storage.RemoveExpense(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete expense"})
		log.Printf("API ERROR: Failed to delete expense: %v\n", err)
		return
	}
	h.removeReceipt(expense.Receipt)
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) DeleteMultipleExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	receipts := make([]string, 0, len(payload.IDs))
	for _, id := range payload.IDs {
		if expense, err := h.storage.GetExpense(id); err == nil && expense.Receipt != "" {
			receipts = append(receipts, expense.Receipt)
		}
	}
	if err := h.storage.RemoveMultipleExpenses(payload.IDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete multiple expenses"})
		log.Printf("API ERROR: Failed to delete multiple expenses: %v\n", err)
		return
	}
	for _, receipt := range receipts {
		h.removeReceipt(receipt)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// UploadReceipt stores a private, locally hosted receipt attachment and returns
// the reference that can be included in an expense payload.
func (h *Handler) UploadReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	file, _, err := r.FormFile("receipt")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Choose a receipt image or PDF up to 5 MB"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 5<<20 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Receipt must be between 1 byte and 5 MB"})
		return
	}
	mimeType := http.DetectContentType(data)
	extensions := map[string]string{
		"image/jpeg":      ".jpg",
		"image/png":       ".png",
		"image/webp":      ".webp",
		"application/pdf": ".pdf",
	}
	ext, ok := extensions[mimeType]
	if !ok {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Receipt must be a JPG, PNG, WebP, or PDF"})
		return
	}
	if err := os.MkdirAll(h.receiptDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Receipt storage is unavailable"})
		return
	}
	name := uuid.New().String() + ext
	if err := os.WriteFile(filepath.Join(h.receiptDir, name), data, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to store receipt"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"receipt": "/receipts/" + name})
}

func (h *Handler) ServeReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
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

func (h *Handler) removeReceipt(reference string) {
	name := strings.TrimPrefix(reference, "/receipts/")
	if name == "" || name != filepath.Base(name) {
		return
	}
	if err := os.Remove(filepath.Join(h.receiptDir, name)); err != nil && !os.IsNotExist(err) {
		log.Printf("WARNING: Failed to remove receipt %s: %v", name, err)
	}
}

// ------------------------------------------------------------
// Recurring Expense Handlers
// ------------------------------------------------------------

func (h *Handler) AddRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	var re storage.RecurringExpense
	if err := json.NewDecoder(r.Body).Decode(&re); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := re.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.storage.AddRecurringExpense(re); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to add recurring expense"})
		log.Printf("API ERROR: Failed to add recurring expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusCreated, re)
}

func (h *Handler) GetRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	res, err := h.storage.GetRecurringExpenses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get recurring expenses"})
		log.Printf("API ERROR: Failed to get recurring expenses: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) UpdateRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	updateAll, _ := strconv.ParseBool(r.URL.Query().Get("updateAll"))

	var re storage.RecurringExpense
	if err := json.NewDecoder(r.Body).Decode(&re); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}
	if err := re.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.storage.UpdateRecurringExpense(id, re, updateAll); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update recurring expense"})
		log.Printf("API ERROR: Failed to update recurring expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) DeleteRecurringExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "ID parameter is required"})
		return
	}
	removeAll, _ := strconv.ParseBool(r.URL.Query().Get("removeAll"))

	if err := h.storage.RemoveRecurringExpense(id, removeAll); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete recurring expense"})
		log.Printf("API ERROR: Failed to delete recurring expense: %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// ------------------------------------------------------------
// Static and UI Handlers
// ------------------------------------------------------------

func (h *Handler) ServeTableView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html")
	if err := web.ServeTemplate(w, "table.html"); err != nil {
		http.Error(w, "Failed to serve template", http.StatusInternalServerError)
	}
}

func (h *Handler) ServeSettingsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html")
	if err := web.ServeTemplate(w, "settings.html"); err != nil {
		http.Error(w, "Failed to serve template", http.StatusInternalServerError)
	}
}

func (h *Handler) ServeStaticFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}
	staticPath := r.URL.Path
	if staticPath == "/manifest.json" {
		staticPath = "/manifest.webmanifest"
	}
	if staticPath == "/sw.js" || staticPath == "/manifest.webmanifest" {
		// PWA metadata and service worker updates should be revalidated so
		// installed clients discover new versions promptly.
		w.Header().Set("Cache-Control", "no-cache")
	}
	if staticPath == "/sw.js" {
		w.Header().Set("Service-Worker-Allowed", "/")
	}
	if err := web.ServeStatic(w, staticPath); err != nil {
		http.Error(w, "Failed to serve static file", http.StatusInternalServerError)
	}
}
