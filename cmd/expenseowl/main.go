package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tanq16/expenseowl/internal/api"
	"github.com/tanq16/expenseowl/internal/storage"
)

var version = "dev"

func main() {
	defaultPort := 8080
	if value := os.Getenv("PORT"); value != "" {
		_, _ = fmt.Sscan(value, &defaultPort)
	}
	port := flag.Int("port", defaultPort, "HTTP port")
	flag.Parse()

	store, err := storage.InitializeStorage()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	handler := api.NewHandler(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handler.ServePage("index.html"))
	mux.HandleFunc("GET /table", handler.ServePage("table.html"))
	mux.HandleFunc("GET /signals", handler.ServePage("signals.html"))
	mux.HandleFunc("GET /settings", handler.ServePage("settings.html"))
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(version)) })
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	mux.HandleFunc("/config", handler.GetConfig)
	mux.HandleFunc("/categories", handler.GetCategories)
	mux.HandleFunc("/categories/edit", handler.UpdateCategories)
	mux.HandleFunc("/category-targets", handler.GetCategoryTargets)
	mux.HandleFunc("/category-targets/edit", handler.UpdateCategoryTargets)
	mux.HandleFunc("/category-parents/edit", handler.UpdateCategoryParents)
	mux.HandleFunc("/expense", handler.AddExpense)
	mux.HandleFunc("/expenses", handler.GetExpenses)
	mux.HandleFunc("/expense/edit", handler.EditExpense)
	mux.HandleFunc("/expense/delete", handler.DeleteExpense)
	mux.HandleFunc("/expenses/delete", handler.DeleteMultipleExpenses)
	mux.HandleFunc("/recurring-expense", handler.AddRecurringExpense)
	mux.HandleFunc("/recurring-expenses", handler.GetRecurringExpenses)
	mux.HandleFunc("/recurring-expense/edit", handler.UpdateRecurringExpense)
	mux.HandleFunc("/recurring-expense/delete", handler.DeleteRecurringExpense)
	mux.HandleFunc("/receipt/upload", handler.UploadReceipt)
	mux.HandleFunc("/receipts/", handler.ServeReceipt)
	for _, path := range []string{"/functions.js", "/dashboard.js", "/table.js", "/signals.js", "/settings.js", "/pwa.js", "/manifest.webmanifest", "/manifest.json", "/sw.js", "/style.css", "/favicon.ico", "/chart.min.js"} {
		mux.HandleFunc(path, handler.ServeStaticFile)
	}
	mux.HandleFunc("/pwa/", handler.ServeStaticFile)

	server := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 90 * time.Second}
	log.Printf("ExpenseOwl %s listening on :%d", version, *port)
	log.Fatal(server.ListenAndServe())
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
