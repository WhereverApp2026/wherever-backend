# Evaluación de la búsqueda semántica — nv-51

> Estado: **medido con IA real el 2026-11-20** (`gemini-embedding-2`). La sección de planes con IA
> (nv-33) está al final: 20 frases contra el servidor de AWS con los 40 negocios reales de Tula.

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
Corrida del 2026-11-20 (el script fecha en UTC), salida sin editar:

Modelo: `gemini/gemini-embedding-2`. Frases: 30. Fecha: 2026-11-21.

| # | Frase | Esperado | Reglas | Semántica | Reglas + afinidad |
|---|---|---|---|---|---|
| 1 | quiero unos tacos al pastor | taqueria-el-fogon | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 2 | algo de comida japonesa | sushi-kaze, ramen-tonkotsu | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 3 | una cena romántica con pasta | la-trattoria | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 4 | comida sin carne ni lácteos | verde-raiz | ❌ | ✅ 1.º | ✅ 1.º |
| 5 | se me antoja un aguachile | mariscos-la-ola | ❌ | ✅ 1.º | ✅ 1.º |
| 6 | un buen corte de carne para ir con mi equipo | asador-don-beto | ❌ | ✅ 1.º | ✅ 1.º |
| 7 | comer barato y casero | fonda-dona-meche | ❌ | ✅ 1.º | ✅ 1.º |
| 8 | desayunar al aire libre el domingo | brunch-la-terraza | ❌ | ✅ 1.º | ✅ 1.º |
| 9 | una sopa caliente porque hace frío | ramen-tonkotsu | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 10 | pizza rápida | pizzeria-napoli, la-trattoria | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 11 | un lugar callado para estudiar con café | cafe-pagina | ✅ | ✅ 1.º | ✅ 1.º |
| 12 | café de especialidad bien preparado | cafe-molienda, cafe-pagina | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 13 | algo dulce para la merienda | pasteleria-azucar | ❌ | ✅ 1.º | ✅ 1.º |
| 14 | quiero ver gatitos mientras tomo algo | cafe-gatuno | ❌ | ✅ 1.º | ✅ 1.º |
| 15 | una bebida sin cafeína | te-y-te | ❌ | ✅ 1.º | ✅ 1.º |
| 16 | unas cervezas con botana y dominó | bar-la-cantina | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 17 | probar destilados de Oaxaca | mezcaleria-agave | ❌ | ✅ 1.º | ✅ 1.º |
| 18 | cerveza local de barril | cerveceria-lupulo | ❌ | ✅ 1.º | ✅ 1.º |
| 19 | cantar con mis amigos | karaoke-estrella | ❌ | ✅ 1.º | ✅ 1.º |
| 20 | tomar un trago viendo el atardecer | rooftop-cielo | ✅ | ✅ 1.º | ✅ 1.º |
| 21 | escuchar música en vivo tranquila | jazz-club-blue | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 22 | ver una película diferente | cine-arte | ❌ | ✅ 1.º | ✅ 1.º |
| 23 | plan con los niños un sábado | museo-ciencia, boliche-strike | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 24 | resolver acertijos en equipo | escape-enigma | ✅ 1.º | ✅ 1.º | ✅ 1.º |
| 25 | algo de adrenalina al aire libre | parque-aventura | ❌ | ✅ 1.º | ✅ 1.º |
| 26 | relajarme después de una semana pesada | spa-calma, yoga-prana | ❌ | ✅ 1.º | ✅ 1.º |
| 27 | empezar a meditar | yoga-prana | ❌ | ✅ 1.º | ✅ 1.º |
| 28 | comprar un regalo hecho a mano | mercado-artesanal | ❌ | ✅ 1.º | ✅ 1.º |
| 29 | buscar una novela | libreria-el-ateneo, cafe-pagina | ✅ | ✅ 1.º | ✅ 1.º |
| 30 | aprender cosas del espacio | museo-ciencia | ✅ | ✅ 1.º | ✅ 1.º |

| Método | Aciertos top-5 | Aciertos top-1 |
|---|---|---|
| Reglas (sin semántica) | 14/30 (47 %) | 10/30 (33 %) |
| Semántica | 30/30 (100 %) | 30/30 (100 %) |
| Reglas + afinidad | 30/30 (100 %) | 30/30 (100 %) |

**Lectura.** La semántica pone el lugar esperado en primer lugar en las 30 frases; las reglas solas
lo logran en 10 y lo dejan fuera del top-5 en 16, justo las que dicen la idea con otras palabras
(«destilados de Oaxaca», «una bebida sin cafeína», «cantar con mis amigos»). El modelo entiende el
significado, no solo las palabras. Reglas + afinidad (lo que usa el asistente) conserva ese 100 %.

## Límites
- El 100 % es sobre un conjunto pequeño y fácil: un negocio claro por nicho. Con 40 negocios reales
  parecidos entre sí (varias fondas o taquerías) los aciertos bajarán; repetirlo con frases de
  usuarios del piloto.
- Con 30 negocios la diferencia puede ser pequeña. Se reporta lo medido.
- El conjunto es ficticio y lo escribió el equipo. Mide comprensión de sinónimos, no el
  gusto real de las personas. Cuando exista el CSV del piloto, se repite con frases de
  usuarios.
- La línea base no usa sinónimos: una frase sin las palabras exactas del giro o de las
  etiquetas depende de la calificación. Ese es justo el caso que la semántica debe mejorar.

## Planes con IA real — nv-33
Corrida del 2026-11-20 contra `https://d302bvc0018nfp.cloudfront.net` (WAF incluido), con los 40
negocios del DENUE de Tula, 4 cuentas de prueba (5 planes cada una por el límite de 10 generaciones
por hora) y 12 s entre planes para no tocar el límite global de 12 llamadas por minuto. Ubicación:
centro de Tula de Allende. Modelo: `gemini-3.5-flash-lite`. Latencia medida desde el cliente.

| # | Frase | Código | Fuente | Paradas | Segundos |
|---|---|---|---|---|---|
| 1 | quiero un plan tranquilo con mi novia para cenar hoy en la noche | 200 | ia | 2 | 2.98 |
| 2 | algo barato para desayunar con mis amigos el sábado | 200 | ia | 1 | 3.10 |
| 3 | llevar a mis papás a comer comida típica de Hidalgo | 200 | ia | 2 | 2.55 |
| 4 | un café bonito para trabajar un rato en la tarde | 200 | ia | 2 | 2.47 |
| 5 | plan con niños el domingo, algo económico | 422 | — | 0 | 1.74 |
| 6 | salir por unas cervezas después del trabajo | 200 | ia | 2 | 2.33 |
| 7 | una cita romántica sin gastar más de 500 pesos | 200 | ia | 2 | 2.50 |
| 8 | tacos y luego un postre con mis primos | 200 | ia | 2 | 2.66 |
| 9 | comer mariscos con la familia el fin de semana | 200 | ia | 2 | 3.09 |
| 10 | quiero conocer la zona arqueológica y luego comer algo | 200 | ia | 2 | 2.61 |
| 11 | estoy aburrido, algo diferente para esta tarde | 200 | ia | 3 | 2.62 |
| 12 | un lugar para festejar el cumpleaños de mi mamá | 200 | ia | 2 | 2.82 |
| 13 | desayuno saludable antes del gimnasio | 200 | ia | 2 | 2.32 |
| 14 | ir por un helado con mi perro | 422 | — | 0 | 1.64 |
| 15 | cena de negocios, un lugar tranquilo y formal | 200 | ia | 1 | 2.44 |
| 16 | algo vegetariano para comer al mediodía | 200 | ia | 2 | 2.60 |
| 17 | tarde de juegos de mesa y botanas con amigos | 200 | ia | 3 | 2.69 |
| 18 | un antojito rápido cerca del centro | 200 | ia | 3 | 3.31 |
| 19 | plan de domingo con mi abuela que usa silla de ruedas | 200 | ia | 2 | 2.31 |
| 20 | quiero pan dulce y un chocolate caliente en la mañana | 200 | ia | 2 | 2.26 |

| Métrica | Resultado |
|---|---|
| Planes generados por la IA | **18/20 (90 %)**; los 18 con `fuente: ia` |
| Descartes a reglas (IA falló, tardó, inválida o inventó un lugar) | **0/20** |
| Sin lugares compatibles (422) | 2/20: «con niños» y «con mi perro». Los negocios del DENUE no traen las etiquetas de apto para niños o mascotas, así que el filtro deja la lista vacía. Es un hueco de datos, no de la IA |
| Respuestas 429 | **0** |
| Latencia del plan (cliente, 18 válidos) | **p50 2,60 s · p95 3,13 s** · máx. 3,31 s |
| Latencia por llamada a Gemini (`usos_ia`, 39 llamadas) | p50 753 ms · p95 926 ms |
| Tokens por plan (2 llamadas: interpretar + itinerario) | ≈1 593 de entrada + 199 de salida |
| Paradas por plan | 1 a 3, media 2,1 |
| Costo | US$0 (capa gratuita de AI Studio) |

**Límite del usuario comprobado de paso:** una sexta petición con la misma cuenta respondió
`fuente: reglas` con su aviso, sin 429, y `usos_ia` la registró como `limite_usuario`.

**Pendiente de datos:** etiquetar en el panel los negocios aptos para niños y mascotas; sin eso,
esas frases seguirán dando 422.
