# plan.md — NV-06 Correo saliente

1. Mapeo
- Crear: `internal/platform/mailer/{smtp.go,async.go,link.go,render.go}`, `internal/platform/mailer/templates/{verificar_correo,recuperar_password,descargar_exportacion}.{html,txt}.tmpl`, pruebas en `internal/platform/mailer/*_test.go` y `tests/{integration,security}/mailer_*_test.go`.
- Modificar: `internal/platform/mailer/mailer.go` (`Name` en `Message`), `internal/platform/config` (claves nuevas, `MAIL_FROM` incluido, y validación de producción), `configs/.env.example`, `deployments/docker-compose.yml` (servicio `mailpit` con perfil `dev`), `internal/testutil/containers.go` (Mailpit en puerto fijo de prueba), `internal/auth/service.go` (nombre en el mensaje; invalidar el token de exportación anterior), `internal/privacidad/{job.go,exports.go}` (aviso síncrono con reintento, `aviso_enviado_en`, tope de 72 h), `cmd/api/main.go` (cola en segundo plano, adaptador directo para el trabajo, apagado ordenado), `docs/asvs-l2-checklist.md`, `Makefile` (`make mail-dev`).
- Sin cambios en `docs/endpoints.md`; sin reordenar el pipeline.

2. Guardarraíles
- `critic B2`: nada de tokens, enlaces ni cuerpos en logs; el cliente SMTP no vuelca la respuesta del servidor.
- `Enqueue` no bloquea; la petición nunca espera a SMTP.
- Producción exige `SMTP_HOST`, TLS y `https://`; falla al arrancar si no.
- Token siempre en el fragmento del enlace (`#token=`).
- CR/LF rechazados en encabezados; plantillas con `html/template`.
- Credenciales SMTP solo por entorno.
- Cada reintento del aviso de exportación invalida el token anterior antes de crear el nuevo.

3. Pruebas
- Las de la spec §Tests. El servidor de correo es Mailpit real; no se simula SMTP salvo el servidor lento o caído de la prueba de respuesta, que es un listener de prueba.

4. Secuencia
1. Configuración y validación al arrancar.
2. Adaptador SMTP, enlaces con fragmento y plantillas.
3. Cola en segundo plano.
4. Conectar auth, registro y cambio de correo (con el nombre).
5. Exportación: aviso síncrono, reintento con token nuevo, `aviso_enviado_en`, tope de 72 h y 24 h desde el aviso.
6. Mailpit en el compose y en las pruebas.
7. Pruebas, ASVS, `Memoria.md`.

5. Verificación
- `go build ./... && go vet ./...`
- `go test ./internal/... ./tests/... -count=1 → verdes`
- `scripts/critic-guardrails.sh → PASS` · `scripts/check_endpoints.sh → OK` · `scripts/check_sdd.sh → SDD-OK`
- `scripts/check_coverage.sh --min 45 → OK` · `semgrep scan --config=auto --config=.semgrep/semgrep.yaml --error → 0`
- Manual: `docker compose --profile dev up`, registrar y ver el correo en `127.0.0.1:8025`.
