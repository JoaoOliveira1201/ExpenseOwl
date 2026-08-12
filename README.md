# ExpenseOwl

ExpenseOwl is a focused, self-hosted monthly expense ledger. It runs as a small Go application backed exclusively by PostgreSQL and ships with a dark, responsive interface.

## Features

- Monthly category breakdown, cashflow, soft category targets, and 12-month trend
- Separate monthly signals page for comparisons and spending pace
- Fast date-bounded queries and indexed PostgreSQL storage
- Expense and income entries with owners, notes, and local receipt attachments
- Searchable transaction ledger with edit and delete actions
- Custom categories and recurring transactions
- CSV import/export; tags in older CSV files are ignored
- Installable PWA with no third-party runtime requests

ExpenseOwl deliberately uses fixed conventions: dark appearance, euro (`EUR`) currency, and calendar months beginning on day 1.

## Run with Docker Compose

1. Create your environment file and choose a strong database password:

   ```sh
   cp .env.example .env
   ```

2. Build and start the application:

   ```sh
   docker compose up -d --build
   ```

3. Open <http://localhost:8080>. Change `APP_PORT` in `.env` if needed.

PostgreSQL data and receipt files live in the named volumes `expenseowl_postgres-data` and `expenseowl_receipt-data`. The app creates and upgrades its schema on startup.

Useful commands:

```sh
docker compose ps
docker compose logs -f app
docker compose down                 # keep data
docker compose down --volumes       # permanently remove all app data
```

## Import a CSV

Start the Compose stack, then run:

```sh
./scripts/import-csv.sh /path/to/expenses.csv
```

For a non-default URL:

```sh
./scripts/import-csv.sh /path/to/expenses.csv https://expenses.example.com
```

Required columns are `name`, `category`, `amount`, and `date` (case-insensitive). Optional columns are `id`, `owner`, `notes`, and `receipt`. Dates may be RFC3339 or `YYYY-MM-DD`. Negative amounts are spending and positive amounts are income. Imports are batched in one PostgreSQL transaction; duplicate IDs are skipped.

The same import and export actions are available on the Manage page.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_PORT` | `8080` | Host port used by Compose |
| `POSTGRES_DB` | `expenseowl` | PostgreSQL database |
| `POSTGRES_USER` | `expenseowl` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `expenseowl` | PostgreSQL password; change this |
| `TZ` | `Europe/Lisbon` | Container timezone |
| `DATABASE_URL` | — | PostgreSQL URL used by the Go process |
| `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE` | — | Standard PostgreSQL connection settings; Compose supplies these |
| `RECEIPT_DIR` | `data/receipts` | Receipt attachment directory |

The application has no built-in authentication. Put it behind an authenticated reverse proxy when exposed outside a trusted network.

## Development

The backend requires Go 1.23 and PostgreSQL. The simplest development loop is still Compose:

```sh
docker compose up --build
```

Run the Go tests with a local Go toolchain:

```sh
go test ./...
```
