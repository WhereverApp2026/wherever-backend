# plan.md — NV-41 Envío de push con Firebase

1. Mapeo
- Crear: `internal/platform/push/` (`fcm.go`, `fcm_test.go`); `internal/notificaciones/push.go` y `push_test.go`; `internal/testutil/fcmtest/`; `tests/integration/push_test.go`.
- Modificar: `internal/notificaciones/service.go` (`WithPush`) y `repository.go` (`RemoveDeviceByHash`); `internal/platform/config/config.go` (`FCM`) y `config_example_test.go` (FCM deja de ser etapa futura); `configs/.env.example`; `cmd/api/main.go` y `internal/testutil/authtest/authtest.go` (`PusherFromConfig`); `Memoria.md`.
- Sin cambios: endpoints, Redis, pipeline e infraestructura (la salida por 443 ya existe; `deploy-env.sh` ya carga todo `/wherever/dev/`).

2. Guardarraíles
- Servicios reales en las pruebas (FCM es interfaz externa: servidor local con llave propia), sin secretos en Git, sin datos personales en logs.
- El token del teléfono y el cuerpo del push nunca van a los logs; el push no lleva nombres ni contenido.
- Un fallo de FCM nunca cambia la respuesta de la acción.

3. Secuencia
1. Preparación: corregir la spec (credenciales en base64).
2. Implementación: cliente FCM, cola, preferencias y horario silencioso, baja de tokens.
3. Cierre: pruebas, documentos, `Memoria.md`, PR; prueba en vivo cuando el equipo cargue las credenciales.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
