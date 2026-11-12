# NV-50 — Embeddings de negocios

## Contexto
Cada negocio recibe un vector de significado calculado con un modelo gestionado. **Origen**: Requisito del curso (ML) y US-035. **Depende de**: NV-20, NV-26, NV-23.

## Requisitos
1. Cada negocio tiene un **vector de significado** (embedding) calculado con un modelo gestionado (Gemini Embedding, `gemini-embedding-2` en `IA_EMBEDDINGS_ID`, por la misma llave de nv-26; Bedrock tiene cuota cero) a partir de nombre, categorías, etiquetas y descripción.
2. Los vectores se calculan al cargar o editar un negocio y se pueden recalcular todos con un comando.
3. Si el servicio de embeddings falla o no hay tope disponible, todo sigue funcionando con el motor de reglas.

## Diseño
- **Almacenamiento (corregido al ejecutar):** colección aparte `vectores_negocios` (`_id` = id del negocio, `vector` de 768 números, `embedding_version`, `modelo`, `actualizado_en`), no un campo en `negocios`: las listas y fichas leen el documento completo y cargarían 768 números por negocio. Sin datos de personas: fuera de ARCO. Los vectores de negocios que dejan de ser aprobados o se eliminan se borran en el siguiente recálculo.
- **Búsqueda (nv-51):** plan B, similitud en la API sobre los vectores de los negocios aprobados (30 en el piloto); el MongoDB local de desarrollo y pruebas no tiene `$vectorSearch`. Atlas Vector Search queda como opción detrás de la misma interfaz, con el índice sobre `vectores_negocios`.
- **Modelo:** `gemini-embedding-2` en `IA_EMBEDDINGS_ID` con la llave de nv-26 (Bedrock tiene cuota cero). 768 dimensiones (`output_dimensionality`); el modelo las normaliza y el adaptador vuelve a normalizar. No acepta `task_type`: el documento va como `title: {nombre} | text: {giro. Etiquetas: … . descripción}` y la consulta como `task: search result | query: {frase anonimizada}`.
- **Texto del negocio** versionado: `embedding_version` = versión de la plantilla + modelo + huella del texto. Si cualquiera cambia, el vector se rehace; un cambio en un campo privado no lo cambia.
- **Cuándo se calcula:** al importar la semilla con cambios (`negocios.Service.WithEmbeddings`, plazo propio de 30 s; si falla, la importación sale igual) y con `make embeddings` (`scripts/recalcular-embeddings`; `TODOS=1` rehace todos). Aún no hay edición de negocios: la de A5 usará el mismo gancho.
- **Llamadas y límites (corregido al ejecutar):** `batchEmbedContents` en lotes de hasta 50 (y no más que el límite por minuto). El modelo de embeddings tiene su propia cuota en Gemini, separada de la del modelo de texto, así que no cuenta contra los límites de nv-28: tiene los suyos, `IA_EMB_LIMITE_MIN` (80) e `IA_EMB_LIMITE_DIA` (800), en Redis (`rate:ia_emb_min`, `rate:ia_emb_dia`); cada texto cuenta uno y sin Redis no se llama. AI Studio no publica la cuota gratuita de `gemini-embedding-2`: son el 80 % de la de `gemini-embedding-001` (100 por minuto, 1000 por día) y se confirman en AI Studio. Con la capa gratuita a costo cero no se registra en `usos_ia`.
- **Privacidad:** a Gemini solo viaja texto público de negocios (nombre, giro, etiquetas y descripción, pasados por `ia.Anonimizar`) y la frase de búsqueda anonimizada, sin identificadores.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Plantilla 4` (`internal/embeddings/plantilla_test.go`): solo campos públicos, sin datos privados ni de control, contacto en la descripción anonimizado, versión que cambia con texto, modelo y plantilla.
- `Unit/Vectores` (`internal/ia/vectores_test.go`): formato de documento y consulta, llave en el encabezado, errores del proveedor, normalización y límites propios (fail-closed sin Redis), contra un servidor local que imita la API (`iatest.ServidorVectores`).
- `Integration/Recalcular 5` (`tests/integration/embeddings_test.go`): solo aprobados e idempotente, solo el desactualizado y todos con la bandera, borra vectores de negocios no visibles, falla o límite sin romper y sigue donde quedó, la importación calcula y sale igual si fallan.
- En vivo, una vez: embeddings reales de los 30 negocios del piloto (`make embeddings` con la llave de SSM).

## Criterios
- Falla sin romper nada.

## Límites
- Confirmar con el docente que embeddings de un modelo gestionado más la evaluación medida de nv-51 cubren el requisito de ML del curso, como se hizo con los scripts de nv-14.
- Con 30 negocios la mejora puede ser pequeña; se reporta lo medido.
- El modelo es de un tercero gestionado; no se entrena uno propio.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11): implementada; Unit, lint y gates locales en verde, el resto en el CI del PR. Falta la prueba en vivo y la firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** colección aparte en lugar de campo, límites propios en lugar de los de nv-28 y los valores 80/800 (a confirmar en AI Studio).
- **Detalle:** la evidencia está en `specs/nv-50/checklists/gates.md` + `Memoria.md`.
