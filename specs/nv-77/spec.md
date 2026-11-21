# NV-77 — Descripción del perfil

## Contexto
La maqueta `82_EditarPerfil` pide una «bio» en el perfil y `81_Perfil` la muestra. Hoy `PATCH /usuarios/yo` solo cambia nombre, usuario, foto y ciudad. **Origen**: inventario de Figma (`wherever-movil/docs/figma-inventory.md`, §4.10) y pedido del revisor (2026-11-21); la hoja A1 del Excel no la trae. **Depende de**: NV-05 (perfil con `If-Match`), NV-70 (perfil de otra persona), NV-42/NV-71 (moderación de texto con reglas).

## Requisitos
1. La persona escribe, cambia o borra una descripción corta de su perfil.
2. La ven quienes pueden ver su perfil (no las cuentas bloqueadas ni inactivas).
3. Un texto con palabras prohibidas no se publica.

## Diseño
- **Campo:** `usuarios.descripcion` (string, opcional, `omitempty`), hasta **160** caracteres (runas). `""` la borra.
- **Escritura:** `PATCH /usuarios/yo` acepta `descripcion` junto a los demás campos (mismo `If-Match` y versión). Se limpia a una línea: caracteres de control y de formato Unicode (incluidas las marcas de dirección `U+202E`, que sirven para suplantar texto) se cambian por espacio y los espacios se juntan.
- **Moderación:** el moderador de textos de nv-42 con las reglas de nv-71 (`moderacion.New(nil, nil, reglas)`). **Rechazado** = 422 `campos[descripcion]: contenido no permitido` y no se guarda nada; dudoso se guarda (mismo criterio que los reportes de negocio: la cuarentena es para contenido que alguien revisa y una bio no entra a la cola).
- **Lectura:** `descripcion` en `GET /usuarios/yo`, en la respuesta del `PATCH`, en la exportación ARCO (usa la misma respuesta) y en `GET /personas/{id}` para quien pueda ver el perfil (mismo 404 para bloqueo, inactiva o propia). No sale en búsquedas, sugerencias ni contactos.
- **Paquetes:** `internal/usuarios` (modelo), `internal/usuarios/account` (`descripcion.go`: limpieza y moderación; `WithModeracion`), `internal/personas` (respuesta del perfil). `cmd/api` y `authtest` conectan el moderador.

## Contratos
| Método | Ruta | Cambio |
|---|---|---|
| PATCH | `/api/v1/usuarios/yo` | acepta `descripcion?` (≤ 160; `""` borra; moderada) |
| GET | `/api/v1/usuarios/yo` | devuelve `descripcion` |
| GET | `/api/v1/personas/{id}` | devuelve `descripcion` |

Sin rutas nuevas (255). Filas actualizadas en `docs/endpoints.md`.

## Tests
- Unit `internal/usuarios/account/descripcion_test.go`: limpieza (saltos, controles, marcas de dirección, emojis) y validación de 160 runas.
- `tests/integration/usuarios_descripcion_test.go`: guardar limpia, verla desde otra persona, borrarla con `""`, más de 160 = 422, palabra prohibida (sin importar mayúsculas) = 422 sin guardar.
- `tests/security/usuarios_descripcion_security_test.go`: 401 ×3; `usuario_id` en el cuerpo no edita a otra persona; marcas de dirección quitadas; se sirve como JSON; con bloqueo el perfil (y la descripción) es 404.
- Contract: `usuarios/yo_200`, `yo_patch_200`, `cancelar_eliminacion_200`, `exportacion_descargar_200` y `personas/perfil_amigo_200` con `descripcion`.

## Criterios
- Solo la dueña cambia su descripción; nadie bloqueado la ve.
- Ninguna palabra prohibida por las reglas queda publicada.

## Límites
- Sin enlaces clicables, menciones ni varias líneas; sin moderación con IA (se puede conectar después con `NewTextoIA`).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-21); implementada en la rama `feat/nv-77-descripcion-perfil`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor (tomadas como las propone la spec):** 160 caracteres en una línea; rechazado = 422 y dudoso se guarda; visible para quien ve el perfil, no en búsquedas.
- **Detalle:** evidencia en `specs/nv-77/checklists/gates.md` + `Memoria.md`.
