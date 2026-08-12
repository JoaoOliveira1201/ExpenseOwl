package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func receiptRequest(t *testing.T, name string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("receipt", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/receipt/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestParseCSVIgnoresRemovedColumns(t *testing.T) {
	csv := "ID,Name,Category,Amount,Date,Tags,Currency,Owner,Notes\nabc,Lunch,Food,-12.50,2026-08-01,work,usd,joao,Team lunch\n"
	expenses, categories, total, skipped, err := parseCSV(strings.NewReader(csv), []string{"Food"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || skipped != 0 || len(expenses) != 1 || len(categories) != 1 {
		t.Fatalf("unexpected import result: total=%d skipped=%d expenses=%#v categories=%#v", total, skipped, expenses, categories)
	}
	if expenses[0].ID != "abc" || expenses[0].Owner != "joao" || expenses[0].Amount != -12.5 {
		t.Fatalf("unexpected expense: %#v", expenses[0])
	}
	if !expenses[0].Date.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected date: %v", expenses[0].Date)
	}
}

func TestUploadAndServeReceipt(t *testing.T) {
	handler := &Handler{receiptDir: t.TempDir()}
	pngHeader := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 504))
	recorder := httptest.NewRecorder()
	handler.UploadReceipt(recorder, receiptRequest(t, "receipt.png", pngHeader))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected upload status %d: %s", recorder.Code, recorder.Body.String())
	}
	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	name := strings.TrimPrefix(result["receipt"], "/receipts/")
	if _, err := os.Stat(filepath.Join(handler.receiptDir, name)); err != nil {
		t.Fatalf("uploaded receipt was not stored: %v", err)
	}

	serveRecorder := httptest.NewRecorder()
	handler.ServeReceipt(serveRecorder, httptest.NewRequest(http.MethodGet, result["receipt"], nil))
	if serveRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected serve status %d", serveRecorder.Code)
	}
	handler.removeReceipt(result["receipt"])
	if _, err := os.Stat(filepath.Join(handler.receiptDir, name)); !os.IsNotExist(err) {
		t.Fatalf("receipt was not removed: %v", err)
	}
}

func TestUploadRejectsExecutable(t *testing.T) {
	handler := &Handler{receiptDir: t.TempDir()}
	recorder := httptest.NewRecorder()
	handler.UploadReceipt(recorder, receiptRequest(t, "receipt.exe", []byte("MZ"+strings.Repeat("x", 510))))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", recorder.Code)
	}
}
