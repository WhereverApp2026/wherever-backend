# Scripts SDD

> Estado: `critic-guardrails.sh` (B1-B4), `check_endpoints.sh` (usa `go run ./scripts/routes`) y `check_sdd.sh` existen desde `nv-01`, `check_coverage.sh` y `check_delta_coverage.py` en `nv-03` (T2).

> `scripts/check_coverage.sh --min 45` gate de cobertura (§7).
> `scripts/check_delta_coverage.py --min 80 --advisory` delta informativo (nv-03).
> YAML validado con `yaml.safe_load`. Tras la mutación siempre `go build ./...` (§8).
