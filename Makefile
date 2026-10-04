.DEFAULT_GOAL := help

GO ?= go
COMPOSE ?= docker compose
# Este endereço é usado pelo migrate dentro da rede do Compose.
DATABASE_URL ?= postgres://goflow:goflow@postgres:5432/goflow?sslmode=disable
MIGRATE = $(COMPOSE) run --rm migrate -path=/migrations -database="$(DATABASE_URL)"

.PHONY: help run build test test-race fmt vet up down logs db-up migrate-create migrate-up migrate-down migrate-version

help: ## Lista os comandos disponíveis
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Executa o worker pool localmente
	$(GO) run ./cmd

build: ## Compila o executável em bin/goflow
	@mkdir -p bin
	$(GO) build -o bin/goflow ./cmd

test: ## Executa os testes
	$(GO) test ./...

test-race: ## Executa os testes com detector de condições de corrida
	$(GO) test -race ./...

fmt: ## Formata o código Go
	$(GO) fmt ./...

vet: ## Executa a análise estática do Go
	$(GO) vet ./...

up: ## Compila e inicia os serviços do Compose
	$(COMPOSE) up -d --build

down: ## Para os serviços preservando o volume do banco
	$(COMPOSE) down

logs: ## Acompanha os logs dos serviços
	$(COMPOSE) logs -f

db-up: ## Inicia o PostgreSQL e espera ficar pronto
	$(COMPOSE) up -d --wait postgres

migrate-create: ## Cria o par de arquivos SQL: make migrate-create NAME=create_jobs
	@test -n "$(NAME)" || { echo "Informe NAME: make migrate-create NAME=create_jobs"; exit 1; }
	$(COMPOSE) run --rm --no-deps migrate create -ext sql -dir /migrations -seq "$(NAME)"

migrate-up: db-up ## Aplica todas as migrations pendentes
	$(MIGRATE) up

migrate-down: db-up ## Reverte somente a última migration
	$(MIGRATE) down 1

migrate-version: db-up ## Mostra a versão e o estado das migrations
	$(MIGRATE) version
