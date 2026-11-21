# plan.md — NV-78 Insignias de las personas

1. Mapeo
- Crear: `internal/insignias/{catalogo,repository,service,handler,privacy}.go`, `internal/personas/insignias.go` y pruebas en las cinco suites.
- Modificar: `internal/personas/{service,handler}.go`, `internal/indexes/indexes.go`, `cmd/api/main.go`, `internal/testutil/authtest/authtest.go`, `scripts/routes/main.go`, `tests/database/indexes_test.go`, `docs/endpoints.md` y la captura de la exportación.

2. Guardarraíles
- Visibilidad de nv-70 para las de otra persona; ARCO completo; conteos con índices existentes; sin ciclo de imports.

3. Secuencia
1. Catálogo y conteos. 2. Servicio, rutas y ARCO. 3. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint; Integration, Security, Database y Contract en local con contenedores y en el CI.
