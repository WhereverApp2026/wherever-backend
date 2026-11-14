# plan.md — NV-56 Panel del negocio: perfil, horarios, pausa, menú y reservas

1. Mapeo
- Crear: `internal/negocios/panel.go` (modelo e interfaces), `internal/negocios/panel/{service,handler,dto,sello,privacy,tarea}.go`, `internal/negocios/menu/{repository,service,handler,dto}.go`, pruebas de las cinco suites.
- Modificar: `internal/negocios/model.go` (`reservas_config`, `cambios_pendientes`, `sello`), interfaces `Observador`, `PanelFuente` y `ReservasFuturas` en `internal/negocios`; `internal/indexes` (lo usan `scripts/indexes` y `/ready`: `ix_menu`, `ix_menu_texto`), `scripts/routes`; rutas en `internal/http`; cola de nv-55 (`estado=cambios_pendientes`); trabajo nocturno (reanudar pausas, reintentar observadores); `internal/privacidad` (Exporter); `docs/endpoints.md` (filas de A5 a la tabla viva al programar). E3 sin biblioteca: `go.mod` sin cambios.
- Sin cambios: el expediente (se llama a la cancelación de nv-55), variables y Terraform.

2. Guardarraíles
- Servicios reales en las pruebas; propiedad validada en el service; el panel nunca lee el expediente.
- Las fuentes del panel y los observadores son interfaces: ningún módulo nuevo se importa desde `negocios`.

3. Secuencia
1. Preparación: decisiones E1–E9; nv-55 en `main`.
2. Implementación: perfil y horarios, pausa con observadores, `reservas-config`, menú, panel y Sello, QR, baja.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
