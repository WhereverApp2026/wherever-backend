# plan.md — NV-20 Negocios aprobados, categorías y carga semilla

## 1. Mapeo
- Crear `internal/negocios/`:
  - `catalog.go`: los 6 giros, nombres de pantalla, dietas, accesibilidad y alérgenos.
  - `model.go`: `Business`, `Direccion`, `Horario`, `VentanaMin`, `Foto`, `Accesibilidad`, `Calificacion`, `Revision`; constantes de estado.
  - `repository.go`: `Collection = "negocios"`, `Indexes()`, `Upsert`, `Facets` y el filtro de visibilidad.
  - `import.go`: parser CSV/JSON y validación pura de `ImportRow` (sin Mongo).
  - `service.go`: importación idempotente, resolución de dueño, `qr_checkin`, `revision` y `Facets`.
  - `dto.go` y `handler.go`: DTOs, rutas, multipart/JSON, errores y `Cache-Control`.
  - `*_test.go`: catálogo, parser y validación (Unit, sin Docker).
- Crear `docs/semilla-negocios.csv`: plantilla con encabezado y 2 filas de ejemplo marcadas (`ejemplo-…`, fotos `https://example.test/…`). El archivo real del piloto no se versiona.
- Modificar:
  - `internal/indexes/indexes.go`: agregar `negocios.Indexes()` a `All()`.
  - `cmd/api/main.go`: construir `negocios.NewRepository(db)` y `negocios.NewService(repo, users, log)`, y montar `negocios.NewHandler(svc, validator)`.
  - `internal/testutil/authtest/authtest.go`: exponer `Negocios *negocios.Service`, montar el handler y agregar `CreateAdmin` (rol `administrador`) para las suites.
  - `internal/testutil/tokens.go`: `ValidTokenFor(t, subject, jti, roles...)` para firmar un admin con `sub` ObjectId; `ValidToken` conserva su comportamiento.
  - `tests/database/indexes_test.go`: agregar `negocios.Collection` a `expected` con `_id_` y los 9 nombres.
  - `docs/endpoints.md`: dos filas nuevas (A2 y A5).
  - `docs/rate-limit-matrix.md`: la fila `Admin` pasa a `nv-20` para `POST /admin/negocios/importar`.
  - `internal/http/openapi.json`: las dos rutas y los esquemas de respuesta; solo dev.
  - `Memoria.md`: decisiones y corrección de drift de esta spec.
- Crear pruebas: `tests/integration/negocios_test.go`, `tests/security/negocios_security_test.go`, `tests/contract/negocios_contract_test.go` y `tests/contract/testdata/negocios/{categorias_200,importar_200}.json`.
- **No crear** `internal/categorias`, `internal/busquedas` (es NV-22), `scripts/indices_negocios.*` ni una colección `categorias`: el catálogo es una constante y los índices los aplica `scripts/indexes`.

## 2. Guardarraíles
- Toda lectura pública filtra en la consulta `estado = "aprobado"` y `eliminado_en` ausente; nunca en memoria después de paginar.
- La semilla es idempotente por `slug`; un error de fila no revierte las filas válidas y nunca duplica.
- El dueño por defecto es el admin que importa; si `propietario_correo` llega, debe existir y estar `activo`. No se escriben correos ni slugs en logs.
- Solo `administrador` usa la importación: `limiter.Window("admin", 200, time.Minute)` antes de `RequireRoles(RoleAdmin)`; el pipeline global no cambia.
- `GET /categorias` no lleva `ETag`, no devuelve 304 y lleva `no-store`; NV-17/NV-18 quedan fuera.
- Sin variables de entorno, dependencias, secretos ni cambios de imagen/pipeline. Los precios son pesos enteros; no se tocan cobros ni datos fiscales.
- `go build ./...` y `go vet ./...` limpios antes de correr suites; Testcontainers con puerto fijo y `-p 1`, una suite a la vez.

## 3. Secuencia
1. Catálogo, modelo y `Indexes()`; registrar en `internal/indexes` y actualizar `tests/database`.
2. Parser CSV/JSON y validación pura con sus Unit; plantilla `docs/semilla-negocios.csv`.
3. Repositorio y servicio de importación (upsert, dueño, QR, revisión, conteos) y `Facets`.
4. Rutas y handler (multipart/JSON, `no-store`, errores), cableado en `cmd/api` y `authtest`.
5. Pruebas Integration, Security y Contract; `testutil.ValidTokenFor` y `CreateAdmin`.
6. Documentos vivos (`endpoints`, `rate-limit-matrix`, `openapi`, `Memoria`) y gates.

## 4. Verificación
- `go build ./...`, `go vet ./...`, `go test ./... -count=1` por suite: Unit, Integration, Security, Database y Contract; `-p 1` en las que levantan contenedores.
- `scripts/critic-guardrails.sh` → PASS; `scripts/check_sdd.sh` → SDD-OK; `scripts/check_endpoints.sh` → OK; `scripts/check_coverage.sh --min 45`; Semgrep 0.
- `scripts/indexes` dos veces sin error; `/ready` 200 con las tres bases y 503 si falta `ux_slug` (probar tras borrar el índice en la base de prueba).
- `curl -s localhost:8080/api/v1/categorias` → 6 categorías y `no-store`; con un admin de prueba, `POST /admin/negocios/importar` con la plantilla → 200 y conteos; repetir → `creados: 0`, `actualizados: N`; un consumidor → 403.
- Importar el CSV real del piloto (20–30) en el entorno de demostración y verificar que `GET /categorias` cuenta solo los aprobados. No versionar ese archivo.

## 5. Riesgos y decisiones abiertas
- El CSV real puede no existir todavía: la plantilla y las pruebas usan datos sintéticos; la carga real es un paso de demostración.
- `alergenos[]` no está en A2; se agrega para que NV-23 pueda filtrar de verdad. Si el revisor prefiere no extender el modelo, NV-23 queda como filtro best-effort y se anota en su spec.
- La semilla re-aprueba al reimportar; una pausa o rechazo manual posterior se pierde si se vuelve a cargar el archivo. Se documenta en la spec de A5.
- `usuarios.negocios_ids` no se mantiene en esta spec: la fuente de verdad para “mis negocios” será `negocios.propietario_id` (`ix_propietario`); A5 decide si sincroniza el arreglo.
- La semilla no verifica documentos: `verificado=false` por defecto; el sello y el plan son de A5.
