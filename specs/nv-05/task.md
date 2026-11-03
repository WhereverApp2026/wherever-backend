# NV-05 — Usuarios, perfil, gustos y privacidad (A1)

## T1 — Validators e índices
- **Crear**: validators (contraseña, mayoría de edad, E.164), campo `version`, índices de `usuarios`.
- **Verificar**: tests de validators y de índices.

## T2 — Registro
- **Crear**: `POST /usuarios`, `GET /usuarios/correo-disponible`, código de amigo, 50 puntos, correo de verificación, tokens.
- **Verificar**: registro→sesión, 409 en carrera, 429.

## T3 — Perfil
- **Crear**: `GET/PATCH /usuarios/yo`, `PUT /usuarios/yo/correo`.
- **Verificar**: `If-Match`, correo no verificado.

## T4 — Ajustes
- **Crear**: gustos, preferencias, privacidad, teléfono (HMAC), modo.
- **Verificar**: reglas de negocio (mínimo 3 categorías, reservas no se apagan, modo sin rol 403).

## T5 — Ciudades
- **Crear**: colección, índices, importación desde CSV/JSON, autocompletado.
- **Verificar**: prefijo, límite, índice usado.

## T6 — ARCO
- **Crear**: `Purger`/`Exporter`, `exportar`, `historial`, `DELETE /usuarios/yo`, `cancelar-eliminacion`.
- **Verificar**: 30 días de gracia, sesiones cerradas, cancelación.

## T7 — Trabajo nocturno
- **Crear**: `gocron` con `lock:borrado_nocturno`, procesamiento de exportaciones.
- **Verificar**: dos instancias, una ejecuta.

## T8 — Docs y gates
- **Modificar**: 15 filas en `docs/endpoints.md`, límites en `rate-limit-matrix.md`, ASVS.
- **Verificar**: `check_endpoints.sh`, `check_sdd.sh`, critic, cobertura, semgrep.
- **Actualizar**: `Memoria.md` con D-2, `gocron` y mediciones.
