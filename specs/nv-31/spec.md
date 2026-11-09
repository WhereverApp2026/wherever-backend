# NV-31 — Interpretar la frase y armar candidatos

## Contexto
La IA convierte una frase libre en parámetros y la API elige los candidatos reales. **Origen**: US-035, US-039, US-040, US-041, US-042, US-043 y hoja A4 del Excel. **Depende de**: NV-20, NV-23, NV-26, NV-29.

## Requisitos
1. `POST /planes/generar` recibe una frase de hasta 300 caracteres y devuelve un **plan sin guardar** (la app lo guarda con `POST /planes`), con `fuente` (`ia` o `reglas`) y `aviso`. El respaldo de nv-33 responde con esta misma forma.
2. **Interpretar (US-041):** la IA convierte la frase en un JSON validado: compañía, presupuesto, hora, ánimo y palabras clave.
3. **Candidatos:** el motor de reglas (nv-23) arma una lista de 15 negocios abiertos y compatibles; solo esos ids se entregan a la IA.

## Diseño
- **Flujo:** frase → `interpretar` (IA) → parámetros → `Recomendar` (nv-23, 15 candidatos) → `armarItinerario` (IA, solo ids candidatos) → `verificar` → `Recalcular` (nv-29) → respuesta.
- **Instrucciones fijas** en archivos versionados (`internal/asistente/prompts/*.txt`); la frase del usuario va solo como entrada.
- **Datos hacia el modelo:** compañía, rango de presupuesto, franja horaria, etiquetas y, de los candidatos, nombre público, categoría, precio y distancia.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/generar` | JWT | 200, 422, 429, 503 |

## Tests
- `Unit/Interpretar 6`: frases completas, frases cortas («relax»), sin presupuesto, texto con instrucciones.

## Criterios
- Cumple los requisitos de arriba.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-31/checklists/gates.md` + `Memoria.md`.
