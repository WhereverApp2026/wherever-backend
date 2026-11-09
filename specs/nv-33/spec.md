# NV-33 — Respaldo, cancelación y «otra idea»

## Contexto
Si la IA falla o tarda, la persona siempre recibe opciones; también puede pedir otro plan o cancelar. **Origen**: US-035, US-039, US-040, US-041, US-042, US-043 y hoja A4 del Excel. **Depende de**: NV-32.

## Requisitos
1. **Respaldo (US-039):** si la IA tarda más de 8 s, falla, se agotó el tope o el límite del usuario, responde el motor de reglas con un plan de la misma forma que nv-31, armado con sus mejores candidatos, `fuente: reglas` y un aviso.
2. **Cancelar (US-040):** si la app corta la conexión, la API cancela la llamada al modelo.
3. `POST /planes/generar/otra-idea` produce un plan distinto excluyendo los lugares ya propuestos.
4. Respuesta promedio por debajo de 5 s con la IA y por debajo de 1 s con el respaldo.

## Diseño
- **Plazo total de 8 s** con un contexto que cancela las dos llamadas a la IA; el motor de reglas corre en paralelo desde el inicio para tener el respaldo listo.
- **Límite y costo:** los de nv-27 y nv-28; una petición que usa el respaldo no cuenta contra el tope de gasto.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/generar/otra-idea` | JWT | 200, 422, 429 |

## Tests
- `Integration/Generar 10` con MongoDB y Redis reales y el servidor local de Converse: plan correcto, la IA inventa un lugar, la IA tarda 9 s (respaldo), la IA falla, tope agotado, límite del usuario, cancelación.
- `Integration/OtraIdea 3`: no repite lugares.
- `Security/Asistente 4`: inyección de instrucciones, frase de 301 caracteres, sin sesión; un plan nunca contiene negocios no aprobados.
- Medición con IA real: 20 frases en español, tasa de descartes, latencia p50 y p95, costo promedio por plan.

## Criterios
- Con la IA caída, el usuario siempre recibe 3 opciones.
- La tasa de descartes queda medida.

## Límites
- La calidad depende del modelo; se mide, no se promete.
- Los traslados son estimados (nv-20).
- Sin memoria de conversación: cada petición es independiente.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-33/checklists/gates.md` + `Memoria.md`.
