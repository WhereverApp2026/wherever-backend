# NV-06 — Correo saliente

## T1 — Configuración
- **Crear**: claves `SMTP_*`, `MAIL_FROM`, `MAIL_*`, `APP_LINK_BASE` en config y `.env.example`; validación de producción (sin `SMTP_HOST`, `SMTP_TLS=none` o `http://` falla).
- **Verificar**: `Unit/Mailer` de configuración, test de `.env.example` de `nv-00`.
- **No hacer**: credenciales reales en el repo.

## T2 — Adaptador SMTP, enlaces y plantillas
- **Crear**: `smtp.go`, `link.go` (token en `#token=`), `render.go` y las tres plantillas en español (texto y HTML) con el nombre escapado; validación de CR/LF; autenticación solo con usuario y TLS.
- **Modificar**: `mailer.Message` con `Name`.
- **Verificar**: pruebas de encabezados, enlaces y render con datos raros.

## T3 — Cola en segundo plano
- **Crear**: `async.go` con cola acotada, trabajadores, tiempo máximo, 2 intentos y apagado ordenado.
- **Verificar**: cola llena, tiempo, reintento, apagado.

## T4 — Conexión con auth y usuarios
- **Modificar**: registro, `reenviar-verificacion`, `recuperar` y cambio de correo para encolar con el nombre.
- **Verificar**: `Integration/Respuesta con servidor lento` y `Integration/Mailer` con Mailpit.

## T5 — Exportación
- **Crear**: `aviso_enviado_en`; aviso síncrono con 2 intentos; cada intento invalida el token anterior y crea uno nuevo; `expira_en = lista_en + 72 h` y, al enviarse el aviso, `min(aviso + 24 h, lista_en + 72 h)`.
- **Verificar**: aviso enviado, reintento tras fallo con token nuevo, vencimiento a las 72 h sin aviso y a las 24 h del aviso.

## T6 — Mailpit
- **Modificar**: `docker-compose.yml` (perfil `dev`), `internal/testutil` (Mailpit en Testcontainers), `Makefile`.
- **Verificar**: correo visible en `127.0.0.1:8025`.

## T7 — Docs
- **Modificar**: `docs/asvs-l2-checklist.md`, `README.md`, `AGENTS.md` si cambia la forma de trabajar.
- **Actualizar**: `Memoria.md` (proveedor sin fijar, deuda de dominio, App Links, token en fragmento para la spec móvil, aviso de privacidad).

## T8 — Gates
- **Verificar**: `critic`, `check_endpoints`, `check_sdd`, cobertura, semgrep, `Security/Mailer`.
