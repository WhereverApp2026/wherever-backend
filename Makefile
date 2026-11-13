# Comandos del proyecto (AGENTS.md §1).
# golangci-lint fijado: v2.15.0 es la primera probada que entiende Go 1.27.
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@v1.9.0
COMPOSE := docker compose -f deployments/docker-compose.yml
VERSION ?= dev

.PHONY: migrar-cifrado deps build vet test test-unit test-integration test-security test-database test-contract contract-capture indexes importar-ciudades embeddings mail-dev lint vuln critic endpoints sdd cover delta mutate mutate-full run infra-up infra-down image check

deps:
	go mod download

build:
	go build ./...
	go build -ldflags "-X main.version=$(VERSION)" -o bin/api ./cmd/api

vet:
	go vet ./...

# Unit no necesita Docker; Integration levanta contenedores con puerto fijo (nunca en paralelo).
test-unit:
	go test ./internal/... ./cmd/... -count=1

test-integration:
	go test ./tests/integration/... -count=1 -p 1

# Security, Database y Contract también levantan contenedores con puerto fijo (-p 1).
test-security:
	go test ./tests/security/... -count=1 -p 1

test-database:
	go test ./tests/database/... -count=1 -p 1

test-contract:
	go test ./tests/contract/... -count=1 -p 1

# Regenera las capturas de tests/contract/testdata; revisar el diff antes de subirlo.
contract-capture:
	CONTRACT_CAPTURE=1 go test ./tests/contract/... -count=1 -p 1

test: test-unit test-integration test-security test-database test-contract

lint:
	$(GOLANGCI_LINT) run ./...

vuln:
	$(GOVULNCHECK) ./...

critic:
	bash scripts/critic-guardrails.sh

endpoints:
	bash scripts/check_endpoints.sh

sdd:
	bash scripts/check_sdd.sh

# Puerta incremental (nv-03). cover mide solo Unit; CI une Unit, Integration y Security.
cover:
	bash scripts/check_coverage.sh --min 45

delta: cover
	python3 scripts/check_delta_coverage.py --min 80 --advisory

# Corren en una copia temporal del árbol; nunca mutan los archivos de trabajo.
mutate:
	bash scripts/mutation.sh

mutate-full:
	bash scripts/mutation.sh --full

# Índices de MongoDB (constitución §5): idempotente, lee MONGO_URI de .env.
indexes:
	go run ./scripts/indexes

# Cifra alergias y fecha de nacimiento (nv-49); sin APLICAR=1 solo cuenta. También rota la llave.
migrar-cifrado:
	go run ./scripts/migrar-cifrado $(if $(APLICAR),-aplicar,)

# Esquema y datos de demostración de Neo4j (nv-11): idempotente, lee NEO4J_URI de .env.
siembra:
	go run ./scripts/siembra

# Rehace AMIGO_DE y FUE_A de Neo4j desde MongoDB (nv-52). Idempotente.
reconstruir-grafo:
	go run ./scripts/reconstruir-grafo

# Vectores de significado de los negocios (nv-50): solo los desactualizados; TODOS=1 rehace todos.
embeddings:
	go run ./scripts/recalcular-embeddings $(if $(TODOS),-todos,)

# Catálogo de ciudades: make importar-ciudades ARCHIVO=ciudades.csv (idempotente).
importar-ciudades:
	go run ./scripts/importar_ciudades $(ARCHIVO)

run:
	go run -ldflags "-X main.version=$(VERSION)" ./cmd/api

infra-up:
	$(COMPOSE) up -d --wait

infra-down:
	$(COMPOSE) --profile dev down

# Correo de desarrollo (nv-06): SMTP en 127.0.0.1:1025 y vista web en http://127.0.0.1:8025.
mail-dev:
	$(COMPOSE) --profile dev up -d mailpit

image:
	docker build --build-arg VERSION=$(VERSION) -t wherever-api:$(VERSION) .

check: build vet test lint critic endpoints sdd
