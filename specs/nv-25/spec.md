# NV-25 — Clima e Inicio

## Contexto
La pantalla de Inicio se arma en una sola llamada con saludo, clima, chips por hora y recomendaciones. **Origen**: US-166, US-025, US-037 y hoja A2 del Excel. **Depende de**: NV-24.

## Requisitos
1. `GET /inicio` arma la pantalla en una llamada: saludo por hora, clima, chips según franja (mañana, tarde, noche) y 5 recomendaciones.
2. `GET /contexto/clima` devuelve el clima de la ciudad o coordenada, con caché de 30 minutos.

## Diseño
- **Clima:** Open-Meteo, sin llave, con tiempo máximo de 3 s.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/inicio` | opcional | 200 |
| GET | `/api/v1/contexto/clima` | no | 200, 422 |

## Tests
- `Integration/Clima 4`: respuesta normal, servicio lento (omite el factor), servicio caído, caché de 30 min.
- `Integration/Inicio 4`: saludo y chips por franja con hora fija; usuario sin sesión recibe lo general.

## Criterios
- Cumple los requisitos de arriba.

## Límites
- El clima depende de un servicio gratuito sin garantía.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 4: plataforma y catálogo completo, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-25/checklists/gates.md` + `Memoria.md`.
