# plan.md — NV-92 Corazón de favorito en las tarjetas

1. Mapeo
- `internal/favoritos` (lectura de marcas por página), `internal/negocios` (interfaz `Favorites`, armado y tarjeta de Explorar), `internal/recomendaciones` (tarjeta de Inicio y Para ti).
- `cmd/api/main.go` y `internal/testutil/authtest` conectan el repositorio de favoritos.
- `docs/endpoints.md`, contratos.

2. Guardarraíles
- Una consulta por respuesta; negocios no importa favoritos (favoritos ya importa negocios): interfaz conectada con `WithFavorites`.
- Sin índices ni variables nuevas.

3. Secuencia
1. Spec y filas de `docs/endpoints.md`.
2. Repositorio, interfaz y armado.
3. Tarjetas de `/lugares`, `/recomendaciones` e `/inicio`.
4. Pruebas, contratos y `Memoria.md`.

4. Verificación
- build, vet, Unit y lint en local (Docker `golang:1.27.3`); Integration, Security, Database y Contract en el CI del PR.
