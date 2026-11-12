# NV-51 — Búsqueda semántica y evaluación

## Contexto
Encontrar lugares por significado y medir si mejora frente a solo reglas. **Origen**: Requisito del curso (ML) y US-035. **Depende de**: NV-50 (vectores en `vectores_negocios`, `ia.Vectorizador`, límites de embeddings).

## Requisitos
1. `GET /lugares/semantica?q=` devuelve los negocios aprobados más cercanos en significado a la frase.
2. El motor de reglas (nv-23) suma un factor `afinidad_semantica` y el asistente (nv-31) lo usa para elegir sus candidatos.
3. Medición de calidad: un conjunto de 30 frases con los lugares esperados; se reportan aciertos entre los 5 primeros (top-5) frente al motor sin semántica.

## Diseño
- **Paquete** `internal/semantica` (handler, DTO, service, repositorio de lectura). No tiene colecciones propias.
- **Índice:** interfaz `semantica.Indice`. Hoy es `ProductoPunto` (plan B): el MongoDB local no tiene `$vectorSearch`, así que la API compara con producto punto contra los vectores de los negocios aprobados (son unitarios: producto punto = coseno). Atlas Vector Search entraría detrás de la misma interfaz. Tope de 1000 negocios comparados.
- **Afinidad:** el coseno se lleva a 0–1 entre el menos y el más parecido del catálogo (mín–máx). Los cosenos de un modelo de embeddings caen en una franja angosta y sin este paso casi no separan.
- **Embedding de la consulta:** una llamada por frase con `ia.Vectorizador.Consulta` (frase anonimizada). El vector se guarda 24 h en Redis, en `cache:ia:emb_{huella}`. La huella es SHA-256 del modelo y de la frase anonimizada y normalizada; no lleva usuario ni frase. Sin Redis no hay caché (no cierra). La llamada cuenta contra los límites de embeddings de nv-50 (`IA_EMB_LIMITE_MIN/DIA`), no contra el tope de gasto de nv-27: ese tope es del modelo de texto y la capa gratuita cuesta 0.
- **Respaldo:** si la IA de embeddings está apagada, no hay cupo, el proveedor falla o ningún negocio tiene vector, responde el motor de reglas con `fuente: reglas` y un aviso. Usa los mismos factores y pesos, con las palabras de la frase (3 letras o más) como gustos. Va sin ubicación y sin filtros duros: un lugar cerrado ahora puede ser el que se busca para luego, y la tarjeta dice `abierto_ahora`.
- **Factor `afinidad_semantica`:** peso 0,20. Los seis pesos de nv-23 se multiplican por 0,8 (presupuesto 0,16, distancia 0,16, horario 0,08, clima 0,08, gustos 0,20, calificación 0,12). Suman 1. Sin semántica el factor vale 0,5 para todos y el orden es el mismo que antes (`TestPesos_SinSemanticaConservaElOrden`). Un negocio sin vector queda neutro.
- **Asistente:** `asistente.Service.WithSemantica` pide la afinidad de la frase dentro del plazo de la IA. `Recomendar` la usa para elegir los 15 candidatos, también en el respaldo por reglas. Si falla, el factor queda neutro.
- **Límite de peticiones:** 30 por minuto por usuario o, sin token, por IP (`rate:semantica:{sujeto}`).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares/semantica?q=&limite=` | opcional | 200, 422, 429 |

- `q` de 2 a 200 caracteres (sin espacios de los extremos); `limite` de 1 a 30, 10 por defecto.
- 200: `{fuente: semantica|reglas, aviso?, items[tarjeta de GET /lugares sin distancia_m, afinidad?]}`, con `Cache-Control: no-store`.

## Tests
- `Unit/Semantica` (`internal/semantica/semantica_test.go`): producto punto, mín–máx, orden y empates, palabras, reglas, híbrido, huella, codificación de la caché y causas del log.
- `Unit/Recomendador` (`puntaje_test.go`): pesos con siete factores, factor neutro sin semántica y orden conservado.
- `Integration/Semántica 8` (`tests/integration/semantica_test.go`) con MongoDB y Redis reales y el servidor local `iatest.ServidorVectores`: frase → esperado primero y solo aprobados; límite; caché de 24 h sin la frase en la llave; proveedor con 500 o 429 → reglas; sin cupo → reglas sin llamar al proveedor; sin vectores → reglas; 422; 429 a la petición 31.
- Medición: `scripts/evaluar-semantica` (30 frases sobre 30 negocios ficticios) da el top-5 de reglas, semántica y reglas + afinidad; se guarda en `docs/ml-evaluacion.md`.

## Criterios
- La búsqueda semántica mejora el top-5 frente a solo reglas, con evidencia medida.
- Hay un informe de evaluación para la defensa.

## Límites
- Con 30 negocios la mejora puede ser pequeña; se reporta lo medido.
- El conjunto de evaluación es ficticio hasta que exista el CSV del piloto (nv-20 T21).
- Plantilla solo en español.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11): implementada; falta la medición con IA real y la firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** pesos con `afinidad_semantica` 0,20 (los demás × 0,8); respaldo sin filtros duros; límite de 30 por minuto.
- **Detalle:** evidencia en `specs/nv-51/checklists/gates.md` + `Memoria.md`.
