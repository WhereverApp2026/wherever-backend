# NV-50 — Embeddings de negocios

## Contexto
Cada negocio recibe un vector de significado calculado con un modelo gestionado. **Origen**: Requisito del curso (ML) y US-035. **Depende de**: NV-20, NV-26, NV-23.

## Requisitos
1. Cada negocio tiene un **vector de significado** (embedding) calculado con un modelo gestionado (Gemini Embedding, `gemini-embedding-2` en `IA_EMBEDDINGS_ID`, por la misma llave de nv-26; Bedrock tiene cuota cero) a partir de nombre, categorías, etiquetas y descripción.
2. Los vectores se calculan al cargar o editar un negocio y se pueden recalcular todos con un comando.
3. Si el servicio de embeddings falla o no hay tope disponible, todo sigue funcionando con el motor de reglas.

## Diseño
- **Almacenamiento:** campo `embedding` (768 dimensiones, `output_dimensionality`; el modelo las normaliza) en `negocios` con **Atlas Vector Search** (incluido en el nivel gratuito, hasta 3 índices); consulta con `$vectorSearch` y filtro `estado = aprobado`.
- **Si el nivel gratuito no permite el índice vectorial:** plan B, calcular la similitud en la API sobre los vectores de 30 negocios (suficiente para el piloto) detrás de la misma interfaz.
- **Texto del negocio** versionado: `embedding_version` para recalcular si cambia la plantilla.
- **Llamadas:** `batchEmbedContents` para recalcular en bloque; la frase de búsqueda con el formato de consulta del modelo. Cuentan contra los límites globales de nv-28 con su propio límite diario (`IA_EMB_LIMITE_DIA`), por debajo del de la capa gratuita.
- **Privacidad:** a Gemini solo viaja texto público de negocios y la frase de búsqueda anonimizada, sin identificadores.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Plantilla 4`: el texto que se vectoriza no incluye datos privados.
- `Integration/Recalcular 4`: idempotente, solo los desactualizados.
- En vivo, una vez: embeddings reales de los 30 negocios del piloto.

## Criterios
- Falla sin romper nada.

## Límites
- Confirmar con el docente que embeddings de un modelo gestionado más la evaluación medida de nv-51 cubren el requisito de ML del curso, como se hizo con los scripts de nv-14.
- Con 30 negocios la mejora puede ser pequeña; se reporta lo medido.
- El modelo es de un tercero gestionado; no se entrena uno propio.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-50/checklists/gates.md` + `Memoria.md`.
