# plan.md — NV-76 Insignia del Sello de Excelencia

1. Mapeo
- Crear: `internal/negocios/panel/insignia.go` (SVG) y sus pruebas en Unit, Integration, Security y Contract.
- Modificar: `internal/negocios/panel/{service,handler,dto}.go` (ruta y servicio), `docs/endpoints.md`, `scripts/routes` no cambia (lee las rutas del armado).
- Sin colecciones, índices ni variables nuevas.

2. Guardarraíles
- Escapar el nombre; CSP en la respuesta SVG; mismo 404 que el resto del panel para ajeno.

3. Secuencia
1. Armado del SVG con pruebas Unit. 2. Servicio, ruta y formatos. 3. Pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, Unit y lint en local; Integration, Security y Contract en local con `--network host` y en el CI del PR; `critic`, `check_endpoints`, `check_sdd`.
