# plan.md — NV-69 Extras de grupos: editar y disolver, punto de encuentro, modo sorpresa y «Me apunto»

1. Mapeo
- Crear: `internal/grupos/{edicion,encuentro,sorpresa,apunto}.go` y sus pruebas en las cinco suites.
- Modificar: `internal/grupos` (modelo con `nombre`, `version`, `eliminado_en`, `punto_encuentro`, `sorpresa` y `ubicacion_compartida`; `RespondRequest`; ARCO; trabajo nocturno que borra ubicaciones vencidas), `internal/planes` (recorte de sorpresa en `Get` y en el paquete de NV-68), `internal/sincronizacion` (mismo recorte), `internal/votaciones` (ocultar votaciones de paradas ocultas), `internal/vaca` (consulta de saldos pendientes), `cmd/api`, `scripts/routes`, `docs/endpoints.md` y `docs/rate-limit-matrix.md`.
- Sin cambios: índices (se reusan `ix_plan` único e `ix_mis_grupos`), variables de entorno, el enlace de NV-38.

2. Guardarraíles
- Servicios reales en las pruebas; ninguna coordenada de integrante en respuestas ni logs; rol y pertenencia validados en el service.
- Sin estado en memoria ni temporizadores: la sorpresa se decide al leer y la carrera de «Me apunto» la resuelve el índice único, porque hay dos instancias detrás del balanceador.
- La política nueva de Redis se mide con `TestPresupuestoRedis_*`.

3. Secuencia
1. Preparación: campos nuevos con valores por omisión y `If-Match` en grupos.
2. Implementación: editar y disolver; ubicación compartida, punto, sugerido y traslados; sorpresa y su recorte en todas las lecturas; «Me apunto».
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
