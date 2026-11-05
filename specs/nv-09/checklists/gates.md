# gates.md — NV-09 CI de seguridad

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos ni cuentas nuevas | salida de `critic` | pendiente |
| critic B2 | Sin tokens en logs | salida de `critic` | pendiente |
| critic B3 | Orden de middlewares HTTP intacto (no revisa CI) | salida de `critic` | pendiente |
| critic B4 | Sin endpoints nuevos | salida de `critic` | pendiente |
| Cobertura | Gate de 45 % intacto | corrida de `ci-pr` | pendiente |
| Línea base | Hallazgos existentes corregidos o exceptuados con motivo y fecha | ver abajo | OK: 0 hallazgos, 0 excepciones nuevas |
| Duración | PR de referencia ≤ 8 min | enlace a la corrida | pendiente |
| Frena | PR con hallazgo plantado sale rojo | enlace a la corrida | pendiente |
| Filtro | PR solo de `docs/` sale verde sin Go ni imagen | enlace a la corrida | pendiente |
| Reporte | Tabla del gate visible en el resumen | captura o enlace | pendiente |
| ZAP | Corrida manual con reporte como artefacto | enlace a la corrida | local OK; falta la corrida en Actions |
| `check_sdd` / `actionlint` | Sin errores | salida | pendiente |

## Línea base (2026-11-05, Go 1.27.3, en contenedores)
- `golangci-lint` v2.15.0: 0. `govulncheck` v1.9.0: 0 alcanzables.
- `gosec` v2.30.0: 0 en 63 archivos, con 7 `#nosec` existentes. En Windows lo bloquea Control de aplicaciones; v2.22.11 no funciona con Go 1.27.
- Semgrep 1.95.0 (`p/golang`, `p/owasp-top-ten`, `p/secrets` y reglas propias): 0 en 202 archivos, con 5 avisos de análisis parcial en bash incrustado (no Go).
- Dockle v0.4.15: sin FATAL (2 INFO: Content Trust y HEALTHCHECK, que distroless no permite sin binario).
- Trivy 0.76.1, HIGH/CRITICAL con corrección: 0 en Debian 12.15, `api` e `indexes`. El registro principal de su base falló en local: se usa también el espejo.
- ZAP 2.17.0, pasivo, local: 35 URL sembradas, 33 rutas recibidas, 0 altas y 1 baja (`Application Error Disclosure` en `/api/v1/probe/error`, que devuelve 500 a propósito y no existe en producción).
- `actionlint` 1.7.7: solo 5 avisos informativos SC2086 en líneas que ya existían; `$COVER` se separa a propósito.
- `scripts/ci/cambios.sh` y `resumen-gate.sh` probados en local: solo documentos, código, workflow, Dockerfile, `.md` en `infra/`, vacío y CRLF; gate con éxito, omitido, fallo, cancelado y sin etapas.
