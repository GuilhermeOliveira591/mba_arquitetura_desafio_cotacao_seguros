GO      ?= go
COMPOSE ?= docker compose
BINDIR  ?= bin

.DEFAULT_GOAL := help

## --- Ambiente (Docker Compose) ---

.PHONY: up
up: ## Sobe o ambiente completo em background (recompila as imagens do starter)
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Derruba o ambiente e remove os volumes
	$(COMPOSE) down -v

.PHONY: ps
ps: ## Mostra o estado dos servicos
	$(COMPOSE) ps

.PHONY: logs
logs: ## Acompanha os logs do ambiente
	$(COMPOSE) logs -f

## --- Aplicacao (Go) ---

.PHONY: build
build: ## Compila os binarios de cmd/ em bin/
	$(GO) build -o $(BINDIR)/ ./cmd/...

.PHONY: run
run: ## Roda a quotation-api localmente (PORT=8080 por padrao)
	$(GO) run ./cmd/quotation-api

.PHONY: test
test: ## Roda os testes
	$(GO) test ./...

.PHONY: fmt
fmt: ## Formata o codigo
	$(GO) fmt ./...

.PHONY: vet
vet: ## Roda a analise estatica do Go
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Sincroniza go.mod e go.sum
	$(GO) mod tidy

.PHONY: clean
clean: ## Remove os binarios compilados
	rm -rf $(BINDIR)

## --- Ajuda ---

.PHONY: help
help: ## Lista os comandos disponiveis
	@awk 'BEGIN {FS = ":.*?## "} \
		/^## ---/ { gsub(/^## /, ""); printf "\n%s\n", $$0; next } \
		/^[a-zA-Z_-]+:.*?## / { printf "  %-8s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
