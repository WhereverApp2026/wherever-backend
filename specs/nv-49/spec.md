# NV-49 — Cifrar los datos sensibles en la base

## Contexto
Alergias y fecha de nacimiento dejan de guardarse en claro, con migración segura. **Origen**: US-226. **Depende de**: NV-48.

## Requisitos
1. Lectura y escritura transparentes para el resto del código mediante una capa en el repositorio.
2. Migración idempotente de los datos que ya estén en claro.
3. El exportador de `nv-05` entrega los datos descifrados al dueño; el purgador los borra.
4. Nada sensible en claro en logs, errores ni copias de seguridad de pruebas.

## Diseño
- **Capa en `usuarios.Repository`** (`internal/usuarios/cifrado.go`, `WithCipher`): al crear (`Create`) y al editar (`apply`, que usan todos los `Update`) cifra `gustos.alergias` (también cuando llega todo `gustos`) y `fecha_nacimiento`; al leer (`findOne`, `apply`) los descifra. El resto del código (`account`, `recomendaciones`, `asistente`, exportación) no cambia.
- **Fecha de nacimiento:** en el modelo, `BirthDate` queda en claro y `BirthDateStored` es lo que va a la base (texto `enc:v1:…`, o la fecha de antes de la migración). Al registrar se guarda `mayor_de_edad: true`, porque la edad ya se validó.
- **Fallo cerrado:** un valor cifrado que no abre (llave faltante, valor alterado o copiado de otra cuenta) es `usuarios.ErrCipher` y la petición responde 503; nunca se devuelve el texto cifrado como si fuera el dato.
- **Sin llave (dev/staging sin `DATA_KEYS`):** se guarda en claro y la API avisa al arrancar (`cifrado_apagado`). Los datos en claro de antes se leen igual en cualquier caso.
- **Migración:** `scripts/migrar-cifrado` (`make migrar-cifrado`, con `APLICAR=1` para escribir) cifra lo que está en claro y re-cifra lo cifrado con una llave que ya no es la activa (rotación). Es idempotente y no sube `version`: no es una edición de la persona.
- **Búsquedas:** estos campos no se buscan ni filtran por valor; el motor de nv-23 los lee ya descifrados.
- **Corrección de drift (2026-11-12):** el ánimo no se guarda (solo el consentimiento); el comando va en `scripts/` como `scripts/indexes`, no en `cmd/`.

## Contratos
Sin endpoints nuevos. Campo nuevo `usuarios.mayor_de_edad`.

## Operación
1. Generar una llave (`openssl rand -base64 32`) y cargar `/wherever/dev/DATA_KEYS` = `v1:<llave>` (SecureString) y, en local, `DATA_KEYS` en `.env`.
2. Desplegar (la siguiente instalación de imagen la toma) y correr `make migrar-cifrado` (cuenta) y `make migrar-cifrado APLICAR=1` contra la base del entorno.
3. Rotar: `DATA_KEYS=v2:<nueva>,v1:<vieja>`, desplegar y volver a correr la migración.

## Tests
- Integration `tests/integration/cifrado_test.go` con MongoDB real: el registro guarda la fecha cifrada y `mayor_de_edad`; alergias cifradas en la base y en claro por la API y la exportación; `gustos.alergias` sola también se cifra; un valor copiado a otra cuenta no abre (`ErrCipher`); migración en modo de prueba, real y por segunda vez (nada pendiente), sin subir `version`; nada sensible en los logs.
- Todas las suites corren con `DATA_KEYS` de prueba (`testutil.DataKeys`): el resto del código sigue igual.

## Criterios
- En la base no hay alergias ni edades en claro (después de migrar).
- El resto del código no cambia.

## Límites
- Sin cifrado de nombre ni correo (no son sensibles según la especificación).
- La llave protege contra el robo de la base, no contra quien tenga acceso a la instancia (nv-48).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; implementada en la rama `nv-49`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** fallo cerrado con 503; sin llave fuera de prod se guarda en claro; la migración no sube `version`.
- **Detalle:** la evidencia irá en `specs/nv-49/checklists/gates.md` + `Memoria.md`.
