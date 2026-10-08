# GoFlow

A Go project for learning background job processing, worker pools, and PostgreSQL persistence.

GoFlow processes jobs asynchronously with PostgreSQL persistence and a Ports & Adapters architecture. Small interfaces are defined in the packages that consume them; the entry point wires the concrete adapters.

The Scheduler queries due pending jobs and sends them through a channel to the worker pool. Workers invoke the Processor with a per-job timeout. The Processor updates job status and calls the Executor, which dispatches to the registered task handler. Successful jobs become `completed`; handler failures become `failed`.

Supported tasks:

- `send_email`: the email handler delegates to LogSender (`log`, logs messages) or Resend (`resend`, sends email through Resend).
- `generate_report`: the report handler validates JSON and requires nonblank `title` and `content`, then delegates to LocalFileWriter (`localfile`).

Each startup creates two temporary demonstration jobs, one of each type. Creation logs confirm persistence; processing happens asynchronously. These demo jobs will be replaced in a future stage. The Application Service and HTTP API have not been implemented.

## Requirements

- Go 1.25 or later
- Docker with Docker Compose
- Make

## Getting started

From the project root, run:

```sh
cp .env.example .env
go mod download
make migrate-up
make run
```

`make migrate-up` starts PostgreSQL, waits for it to become ready, and applies pending migrations using golang-migrate in Docker. No local migrate installation is required.

The application connects to `localhost:5432` using `goflow` as the database name, username, and password. These settings are intended for local development.

## Environment configuration

Configuration lives in `internal/config`. The application loads `.env` from the working directory through `config.Load()`.
Copy `.env.example` to `.env`, adjust the values, and run `make run`.
Process environment variables take precedence over `.env`; missing files are allowed.
The loader does not change the process environment. It supports `KEY=value`, optional
`export`, single or double quotes, and comments. Values are literal, without shell
commands, escape processing, or variable expansion. `.env.example` is only a template.
Defaults apply only to unset variables; explicitly empty settings are rejected,
except for email credentials when the provider is not `resend`.

| Variable | Local default |
| --- | --- |
| `DATABASE_URL` | `postgres://goflow:goflow@localhost:5432/goflow?sslmode=disable` |
| `WORKER_COUNT` | `3` |
| `WORKER_JOB_TIMEOUT` | `30s` |
| `SCHEDULER_BATCH_SIZE` | `100` |
| `SCHEDULER_POLL_INTERVAL` | `5s` |
| `EMAIL_PROVIDER` | `log` |
| `RESEND_API_KEY` | Unset |
| `EMAIL_FROM` | Unset |
| `REPORT_PROVIDER` | `localfile` |
| `REPORT_OUTPUT_DIR` | `./output` |

Counts and durations must be greater than zero. Durations use Go syntax,
such as `250ms`, `30s`, or `1m30s`.

With `EMAIL_PROVIDER=resend`, both `RESEND_API_KEY` and `EMAIL_FROM` are required.
Configuration loading does not construct adapters or send email. The entry point constructs the `log` or `resend` adapter and injects it into
the email task handler. Unsupported providers cause a startup error. The `log`
adapter prints the message without requiring credentials or sending email.

## Report output

`REPORT_PROVIDER=localfile` selects the currently supported report adapter.
`REPORT_OUTPUT_DIR` selects the output directory, defaulting to `./output` relative
to the application's working directory. LocalFileWriter creates the directory
when needed and writes `<JobID>.txt` containing the title, a blank line, the body,
and a final newline. Invalid IDs, including path separators and traversal attempts,
are rejected. A context canceled before writing prevents filesystem changes.

Writing the same Job ID again overwrites and truncates the existing file using
`os.WriteFile`. Writes are not atomic or versioned, and cancellation does not
interrupt an ongoing filesystem operation. Retry/backoff, distributed claiming,
leases/recovery, and idempotency remain future work.

## Commands

| Command | Description |
| --- | --- |
| `make help` | List available commands |
| `make run` | Run the application locally |
| `make build` | Build the executable at `bin/goflow` |
| `make test` | Run all tests, including PostgreSQL integration tests |
| `make test-race` | Run tests with the race detector |
| `make fmt` | Format Go code |
| `make vet` | Run Go static analysis |
| `make db-up` | Start PostgreSQL and wait for it to become ready |
| `make down` | Stop Compose services while preserving database data |

## Migrations

Create a migration:

```sh
make migrate-create NAME=add_job_index
```

Edit the generated `.up.sql` and `.down.sql` files in `migrations/`, then run:

```sh
make migrate-up       # Apply pending migrations
make migrate-version  # Show the current migration version
make migrate-down     # Revert the last migration
```

Reverting a migration may remove data, depending on its SQL.

## Tests

The report tests use a fake Writer and temporary directories; they do not require
PostgreSQL. Run them directly with:

```sh
go test ./internal/tasks/report/... ./internal/adapter/report/localfile/...
```

Integration tests require a running PostgreSQL database with the migrations applied:

```sh
make migrate-up
make test
```

They use the local development database by default. To use a separate database, apply the migrations to it first and set `TEST_DATABASE_URL`:

```sh
TEST_DATABASE_URL='postgres://user:password@localhost:5432/goflow_test?sslmode=disable' make test
```

## Project structure

```text
cmd/                       Application entry point
internal/job/              Job model, handlers, executor, and repository interface
internal/config/           Environment and .env configuration
internal/scheduler/        Due-job polling and dispatch
internal/worker/           Worker pool implementation
internal/processor/        Job execution and status transitions
internal/tasks/            Email and report handlers and consumer-defined ports
internal/adapter/email/    LogSender and Resend adapters
internal/adapter/report/   LocalFileWriter adapter
internal/storage/postgres/ PostgreSQL repository and integration tests
migrations/                SQL migrations
docs/                      Learning notes and migration instructions
```
