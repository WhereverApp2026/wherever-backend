# plan.md — NV-93 Lista de cobros del negocio

1. Mapeo
- `internal/pagos` (repositorio, service, DTO, ruta e índice).
- `docs/endpoints.md`; `tests/integration`, `tests/security`, `tests/database` y `tests/contract`.

2. Guardarraíles
- Propiedad en el service; sin datos de quien pagó; cursor firmado con negocio y filtros; sin variables nuevas.

3. Secuencia
1. Spec y fila de `docs/endpoints.md`.
2. Índice y consultas del repositorio.
3. Service, DTO, handler y ruta.
4. Pruebas, documentos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local (Docker `golang:1.27.3`); Integration, Security, Database y Contract en el CI del PR.
