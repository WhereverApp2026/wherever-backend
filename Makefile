# Comandos del proyecto (AGENTS.md §1). infra-up/infra-down llegan en nv-01.
# golangci-lint fijado: v2.15.0 es la primera probada que entiende Go 1.27.
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
.PHONY: deps build vet test lint critic check

deps:
	go mod download

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./... -count=1

lint:
	$(GOLANGCI_LINT) run ./...

critic:
	bash scripts/critic-guardrails.sh

# Gates de nv-00 en vacío.
check: build vet test lint critic
