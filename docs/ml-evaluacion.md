# Evaluación de la búsqueda semántica — nv-51

> Estado: **pendiente de medir con IA real.** El script y el conjunto están listos. El perfil
> local `wherever-dev` no puede leer SSM (`ssm:GetParameter` denegado, 2026-11-11), así que
> no hubo llave para la corrida real. Las tablas de abajo se llenan con la salida del script.

## Qué se mide
- **Pregunta:** ¿la búsqueda por significado pone el lugar correcto entre los 5 primeros más
  seguido que el motor de reglas solo?
- **Conjunto:** `scripts/evaluar-semantica/conjunto.json`. Son 30 negocios ficticios (el
  piloto aún no tiene su CSV real, nv-20 T21), en los seis giros, y 30 frases en español,
  cada una con uno o dos lugares esperados. Casi todas las frases dicen la idea con otras
  palabras («destilados de Oaxaca» → mezcalería). Así se ve si el modelo entiende el
  significado o solo coincide con palabras.
- **Métodos** (los tres con los pesos versionados de `internal/recomendador`):
  1. **Reglas:** el motor de nv-23 con las palabras de la frase como gustos. Es la respuesta de
     `GET /lugares/semantica` cuando la IA no está.
  2. **Semántica:** orden por afinidad (coseno de `gemini-embedding-2`, llevado a 0–1 entre
     el menos y el más parecido). Es la respuesta normal del endpoint.
  3. **Reglas + afinidad:** el motor con el factor `afinidad_semantica` (peso 0,20). Así elige
     el asistente sus 15 candidatos (nv-31).
- **Métricas:** acierto top-5 (algún esperado entre los 5 primeros) y acierto top-1.
- **Control:** lunes 13:00 en Ciudad de México, todos abiertos y sin ubicación. Así el
  horario y la distancia no deciden.

## Cómo correrlo
Son 31 llamadas a Gemini: un lote con los 30 negocios y una por frase, con una pausa de
1,5 s. No usa MongoDB ni Redis. La llave va del almacén al entorno del contenedor sin
imprimirse. Hace falta un perfil con `ssm:GetParameter` sobre `/wherever/dev/GEMINI_API_KEY`.

```bash
GEMINI_API_KEY=$(aws ssm get-parameter --name /wherever/dev/GEMINI_API_KEY --with-decryption --query Parameter.Value --output text) IA_EMBEDDINGS_ID=gemini-embedding-2 MSYS_NO_PATHCONV=1 docker run --rm -e GEMINI_API_KEY -e IA_EMBEDDINGS_ID -v "$(pwd -W):/src" -v gomod:/go/pkg/mod -w /src golang:1.27.3 go run ./scripts/evaluar-semantica > evaluacion.md
```

Pega la salida en la sección de resultados. Solo viajan las frases del conjunto (anonimizadas
por `ia.TextoConsulta`) y textos públicos de los negocios ficticios.

## Resultados
_Pendiente._

| Método | Aciertos top-5 | Aciertos top-1 |
|---|---|---|
| Reglas (sin semántica) | — | — |
| Semántica | — | — |
| Reglas + afinidad | — | — |

## Límites
- Con 30 negocios la diferencia puede ser pequeña. Se reporta lo medido.
- El conjunto es ficticio y lo escribió el equipo. Mide comprensión de sinónimos, no el
  gusto real de las personas. Cuando exista el CSV del piloto, se repite con frases de
  usuarios.
- La línea base no usa sinónimos: una frase sin las palabras exactas del giro o de las
  etiquetas depende de la calificación. Ese es justo el caso que la semántica debe mejorar.
