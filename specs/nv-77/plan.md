# plan.md — NV-77 Descripción del perfil

1. Mapeo
- Crear: `internal/usuarios/account/descripcion.go` y pruebas en las cuatro suites.
- Modificar: `internal/usuarios/repository.go` (campo), `internal/usuarios/account/{dto,service}.go`, `internal/personas/perfil.go`, `cmd/api/main.go` y `internal/testutil/authtest/authtest.go` (moderador), `docs/endpoints.md`, capturas de contrato.
- Sin colecciones, índices, rutas ni variables nuevas.

2. Guardarraíles
- Moderación con las reglas reales; texto limpio de controles y marcas de dirección; mismo 404 del perfil para bloqueo.

3. Secuencia
1. Campo, DTO y limpieza con Unit. 2. Moderación y respuestas. 3. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit, lint; Integration, Security y Contract en local con contenedores y en el CI.
