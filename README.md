# ExpenseOwl

ExpenseOwl is a focused, self-hosted monthly expense ledger. It runs as a small Go application backed exclusively by PostgreSQL and ships with a cozy, responsive interface.

## Features

- Monthly category breakdown, cashflow, soft category targets, and 12-month trend
- Separate monthly signals page for comparisons and spending pace
- Fast date-bounded queries and indexed PostgreSQL storage
- Expense and income entries with owners, notes, and local receipt attachments
- Searchable transaction ledger with edit and delete actions
- Custom categories and recurring transactions
- Installable PWA with no third-party runtime requests
- Optional token-protected MCP endpoint for Hermes and other agents

ExpenseOwl deliberately uses fixed conventions: a cozy light appearance, euro (`EUR`) currency, and calendar months beginning on day 1.

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

## Deploy updates

Run the root deployment script from the server checkout:

```sh
./deploy.sh
```

It requires a clean working tree, fetches and fast-forwards the current branch from `origin`, rebuilds the app image, starts the Compose stack, and prints container status.

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
| `MCP_TOKEN` | — | Bearer token for `/mcp`; unset or blank disables MCP |

The web interface and existing HTTP APIs have no built-in authentication. Put them behind an authenticated reverse proxy when exposed outside a trusted network.

## Connect Hermes with MCP

ExpenseOwl serves MCP over Streamable HTTP at `/mcp` on the application's existing port. It uses the official Go MCP SDK, pinned to the release compatible with Go 1.23. No separate process or database access is needed on the agent host.

1. Generate a random token on the ExpenseOwl host, save it as `MCP_TOKEN` in the private `.env` file, and rebuild/restart the app:

   ```sh
   openssl rand -hex 32
   docker compose up -d --build app
   ```

2. On the Hermes host, store the same token as `EXPENSEOWL_MCP_TOKEN` in Hermes's private `~/.hermes/.env`. Merge this entry into `~/.hermes/config.yaml`, keeping any existing MCP servers:

   ```yaml
   mcp_servers:
     expenseowl:
       url: "http://YOUR_EXPENSEOWL_HOST:8080/mcp"
       headers:
         Authorization: "Bearer ${EXPENSEOWL_MCP_TOKEN}"
       skip_preflight: true
   ```

   Use the app's reachable host and configured port. `localhost` refers to the Hermes host when Hermes runs on a different machine. `skip_preflight` allows the stateless endpoint's normal `405` GET response; the actual MCP connection and authentication still run. Keep Hermes secrets and runtime configuration on its own host.

3. Reload MCP with `/reload-mcp` in Hermes, or restart Hermes. Hermes exposes tools with names such as `mcp__expenseowl__add_expense`. The [Hermes MCP reference](https://hermes-agent.nousresearch.com/docs/reference/mcp-config-reference) describes configuration and secret substitution.

Available tools:

| Tools | Purpose |
| --- | --- |
| `get_config` | Read categories, budgets, category groups, and allocation targets |
| `list_expenses`, `get_expense` | Read transactions, with dates, owners, notes, and receipt references |
| `add_expense`, `update_expense`, `delete_expense` | Create, partially update, or delete a transaction |
| `list_recurring_expenses`, `add_recurring_expense`, `update_recurring_expense`, `delete_recurring_expense` | Manage schedules and generated transactions |
| `update_categories`, `update_category_targets`, `update_category_parents`, `update_allocation_targets` | Change ledger settings |

Amounts are in EUR: **negative means spending; positive means income**. Spending needs a category; income has no category. Dates use RFC3339 timestamps with a timezone, such as `2026-10-02T12:00:00+01:00`. Omitted owners default to `common`. Expense lists default to 100 items, allow up to 200, and return `nextCursor` when another page exists. Pass that value as `cursor`; follow all pages when calculating totals. `from` is inclusive and `to` is exclusive.

Transaction and recurring updates change only supplied fields. Existing receipt attachments remain attached to edited transactions; deleting a transaction also deletes its attachment. Recurring updates/deletions affect future generated entries by default. `updateAll: true` or `removeAll: true` also replaces or deletes historical generated entries. Settings tools replace their corresponding list or mapping, so read `get_config` first to preserve other values. Receipt uploads remain available through the web interface.

Every MCP request requires the token, which grants all listed tools, including writes and deletes. The MCP endpoint rejects browser-origin requests. Keep it on a trusted private network or use HTTPS. MCP authentication covers `/mcp`; the web interface and existing HTTP endpoints still rely on your network or reverse-proxy access controls.

## Development

The backend requires Go 1.23 and PostgreSQL. The simplest development loop is still Compose:

```sh
docker compose up --build
```

Run the Go tests with a local Go toolchain:

```sh
go test ./...
```
