# GoFlow

A Go project for learning background job processing, worker pools, and PostgreSQL persistence.

The project includes job handlers, a job executor, concurrent workers, and a PostgreSQL repository. The current application saves a sample job and reads it back from the database.

## Requirements

- Go 1.25 or later
- Docker with Docker Compose
- Make

## Getting started

From the project root, run:

```sh
go mod download
make migrate-up
make run
```

`make migrate-up` starts PostgreSQL, waits for it to become ready, and applies pending migrations using golang-migrate in Docker. No local migrate installation is required.

The application connects to `localhost:5432` using `goflow` as the database name, username, and password. These settings are intended for local development.

## Environment configuration

The application loads configuration from environment variables through `internal/config`.
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

Counts and durations must be greater than zero. Durations use Go syntax,
such as `250ms`, `30s`, or `1m30s`.

With `EMAIL_PROVIDER=resend`, both `RESEND_API_KEY` and `EMAIL_FROM` are required.
Configuration loading does not construct adapters or send email. The current
entry point still uses the demonstration email handler; the Resend adapter and
provider selection will be wired separately.

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
internal/worker/           Worker pool implementation
internal/storage/postgres/ PostgreSQL repository and integration tests
migrations/                SQL migrations
docs/                      Learning notes and migration instructions
```
