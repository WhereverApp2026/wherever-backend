# NV-38 — Enlace de invitación y vista pública

## Contexto
Quien no tiene la app puede ver el plan y entrar con un enlace seguro. **Origen**: US-098, US-099, US-237 y hojas J3 y A4 del Excel. **Depende de**: NV-37.

## Requisitos
1. **Enlace:** `POST /planes/{id}/compartir` crea un código aleatorio de 128 bits con vencimiento de 7 días, revocable.

## Diseño
- **Código del enlace:** se guarda solo el hash (SHA-256); el código no es adivinable.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/{id}/compartir` | JWT dueño | 201 |
| GET | `/api/v1/planes/compartidos/{codigo}` | no | 200, 404, 429 |
| POST | `/api/v1/grupos/unirse/{codigo}` | JWT | 200, 404, 409, 410 |

## Tests
- `Security/Grupos 6`: vista pública sin datos personales; ajeno recibe 404; el código no se guarda en claro; límite por IP del enlace.

## Criterios
- La vista pública no revela personas.

## Límites
- La vista web mínima solo sirve datos; la página la pone la app o un sitio estático.
- Los Android App Links los configura la app.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 5: grupos y notificaciones, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-38/checklists/gates.md` + `Memoria.md`.
