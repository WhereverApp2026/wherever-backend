# NV-32 — Itinerario con IA y verificación contra el catálogo

## Contexto
La IA propone 2 a 4 paradas y la API descarta cualquier lugar que no exista o esté cerrado. **Origen**: US-035, US-039, US-040, US-041, US-042, US-043 y hoja A4 del Excel. **Depende de**: NV-31 (`internal/asistente`, `POST /planes/generar`) y NV-29 (`planes.Recalcular`).

## Requisitos
1. **Itinerario (US-042):** la IA elige de 2 a 4 paradas de los 15 candidatos del motor, en orden horario, con hora de llegada (`HH:MM`) y estancia sugerida (15 a 240 min), dentro del presupuesto por persona.
2. **Verificación (US-043):** la API descarta cualquier parada con un id fuera de los candidatos (inventado, de un negocio que el motor filtró por alergia o dieta, o cerrado), repetida, con la hora antes de poder llegar o con más de 60 min de espera, cerrada al llegar o al salir, o que rompe el presupuesto. Cada descartada se reemplaza al final con el siguiente candidato del motor (mismas reglas que el armado de nv-31) y cada descarte se registra.
3. Cada parada trae su `por_que` (nv-23).

## Diseño
- **Flujo** (`internal/asistente`): frase → `Interpretar` (IA, nv-31) → `recomendador.Recomendar` (15) → `GenerarItinerario` (IA) → `Verificar` → `planes.Recalcular`. Sin candidatos no se llama a la IA (422).
- **Petición del paso 2:** instrucciones fijas en `prompts/itinerario.txt` (embebido); en el contexto, inicio (día y hora), presupuesto por persona y los candidatos (id, nombre, giro, precio y horario de ese día); en el turno de usuario, solo lo que se entendió de la frase (compañía, ánimo con consentimiento, palabras clave), nunca la frase. Sin ubicación, perfil ni ids de personas.
- **Validación:** `Itinerario.Validar` revisa solo la forma (2 a 4 paradas, `HH:MM`, minutos); un error se reintenta una vez (`ia.GenerarValidado`). Los ids, horarios y presupuesto los revisa `Verificar`, que reemplaza en lugar de reintentar.
- **Horas:** se conservan la hora y la estancia de la IA si pasan la verificación. Una hora más de 12 h antes del inicio es del día siguiente (plan que cruza la medianoche).
- **Descartes:** colección `ia_descartes` (`motivo`, `negocio_id` si el id es un ObjectID, `modelo`, `creado_en`, `expira_en`), TTL de 30 días (`ttl_descarte` sobre `expira_en`) e índice `ix_motivo_fecha`. Sin texto ni id del usuario: no es dato personal y no entra a ARCO. Si la base falla, el plan sale igual y solo se avisa en el log. Motivos: `fuera_de_candidatos`, `repetido`, `cerrado`, `presupuesto`, `orden_horas`.
- **Errores de la IA en el paso 2:** los mismos de nv-31 (429 con `Retry-After`, 503). El respaldo por reglas es de nv-33.
- **Ánimo y alergias:** el ánimo solo viaja con `privacidad.estado_animo`; las alergias y la dieta del perfil nunca viajan y siempre actúan como filtro duro de los candidatos.

## Contratos
Sin endpoints nuevos. `POST /api/v1/planes/generar` (nv-31) conserva su forma; su fila en `docs/endpoints.md` anota nv-32.

## Tests
- `Unit/Verificar` (`internal/asistente/verificar_test.go`): itinerario válido, id inventado, id que no es ObjectID, cerrado a esa hora, presupuesto excedido, orden de horas roto (antes de acabar la anterior, hueco de más de 60 min y antes del inicio), repetido, cruce de medianoche, todo descartado (422 con descartes registrados); `Validar` de forma, reintento y la petición solo con datos públicos.
- `Integration/Asistente` (`tests/integration/asistente_test.go`): la IA propone el negocio con el alérgeno del perfil y un id inventado; ambos se descartan, quedan en `ia_descartes` y el motor completa el plan. Ninguna petición lleva datos de la persona. Ánimo solo con consentimiento. Itinerario inválido dos veces = 503; sin candidatos no hay segunda llamada.
- `Database/Indexes`: `ia_descartes` con `ttl_descarte` e `ix_motivo_fecha`.

## Criterios
- El plan devuelto solo tiene lugares reales, abiertos y dentro del presupuesto.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11); implementada en la rama `nv-32`, PR contra `nv-31`. Falta la firma y el CI del PR (Integration, Security, Database, Contract).
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** espera máxima de 60 min entre paradas; reemplazo al final del plan (no en la posición de la descartada); nombres de negocios hacia el modelo (dato público).
- **Detalle:** evidencia en `specs/nv-32/checklists/gates.md` + `Memoria.md`.
