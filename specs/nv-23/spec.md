# NV-23 — Filtros duros y puntuación de recomendaciones

## Contexto
El motor recomienda desde el primer día sin datos de entrenamiento y es el plan B de la IA. **Origen**: US-166, US-025, US-037 y hoja A2 del Excel. **Depende de**: NV-20, NV-05.

## Requisitos
1. **Filtros duros** (nunca se recomienda lo que los incumple): negocio no aprobado, cerrado a esa hora, alergias y dieta del usuario, accesibilidad, mascotas, niños, y los marcados con «No me interesa».
2. **Puntuación ponderada** de 0 a 100 por presupuesto, distancia, horario, clima, gustos y calificación.
3. Cada recomendación incluye `razones[]` (los factores que más sumaron) y `por_que`, una línea en español legible (US-037).

## Diseño
- **Paquete** `internal/recomendador`: `Filtro` (reglas duras), `Puntaje` (una función por factor, con su peso), `Explicador` (arma `por_que` con los dos o tres factores principales).
- **Pesos** en `configs/pesos_recomendador.json`, validado al arrancar (suman 1,0).
- **Factores:** presupuesto (1 dentro del rango, cae linealmente hasta 0 al doble), distancia (decae con la distancia al uso previsto), horario (abierto con margen de 30 min antes del cierre), clima (lluvia favorece techado; calor favorece helados y terrazas; se define por etiquetas), gustos…
- **Determinista:** misma entrada, mismo resultado; los empates se rompen por calificación y luego por id.
- **Reutilizable:** `Recomendar(ctx, contexto, n)` es una función de paquete; el asistente (nv-31) la usa para armar candidatos y como plan B.
- **Clima:** el clima llega con nv-25, pospuesta. Hasta entonces el factor vale 0,5 para todos y no cambia el orden.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Filtros 10`: cada filtro duro por separado y combinados; alergia nunca se cuela.
- `Unit/Puntaje 12`: casos fijos con resultado esperado por factor; pesos que no suman 1 hacen fallar el arranque.
- `Unit/Explicador 5`: elige los factores correctos; texto sin datos sensibles.

## Criterios
- Con 10 negocios fijos el orden coincide con el esperado por el equipo.
- Ninguna recomendación incumple una alergia declarada.

## Límites
- Sin aprendizaje automático: los pesos son fijos (el ML llega con nv-50 y nv-51).
- La distancia es en línea recta.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 1: catálogo y recomendaciones
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-23/checklists/gates.md` + `Memoria.md`.
