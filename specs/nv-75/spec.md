# NV-75 — Consejo del día y descripción con IA para el negocio

## Contexto
El dueño recibe en P-47 un consejo del día («Los martes de 4 a 6 tienes poca gente. ¿Lanzas un cupón?», US-142) y en el alta o el perfil puede pedir «Que la IA me ayude» con la descripción a partir de 3 palabras. **Origen**: hoja «A5 Negocios aliados» del Excel `Wherever_Endpoints_CRUD_v3.xlsx` (dos filas de endpoints). **Depende de**: NV-04/NV-05 (sesión, `RequireActive`), NV-11 (presupuesto de Redis), NV-26 (`ia.Modelo` con Gemini, salida validada con esquema), NV-27 (`ia.Guardia`: tope de gasto y caché), NV-28 (límites por usuario y globales, anonimizado), NV-56 (propiedad del negocio), NV-61 (cupones), NV-62 (reseñas por aspecto) y NV-74 (`estadisticas_diarias`).

## Requisitos
1. `GET /negocios/{id}/consejo` da un consejo con su acción, uno por negocio y día; siempre responde, con IA o sin ella.
2. `POST /negocios/{id}/ia/descripcion` redacta una descripción de hasta 300 caracteres a partir de 3 palabras.
3. Al modelo solo le llegan agregados del negocio: nunca nombres, correos, ids de clientes ni textos de reseñas.
4. La IA del negocio no puede dejar sin cuota a los planes de los consumidores (Gemini gratis: 15 RPM y 500 RPD; la app usa 12 y 400).

## Diseño
### Consejo del día
- **Quién:** JWT `comerciante` + `RequireActive`, dueño del negocio (si no, 404).
- **Hechos (sin IA):** un motor de reglas en `internal/consejos` arma candidatos con datos agregados de los últimos 28 días y elige el de mayor peso:
  - franja floja: día de la semana y bloque de 2 h con menos llegadas frente al promedio del negocio (de `estadisticas_diarias.por_hora`, nv-74) → acción `crear_cupon`;
  - aspecto en baja: el aspecto de nv-62 que más bajó contra el mes anterior → `ver_resenas`;
  - reseñas sin respuesta en los últimos 14 días (conteo) → `responder_resenas`;
  - cupón activo sin canjes en 14 días → `editar_cupon`;
  - reservas apagadas con visitas altas → `configurar_reservas`;
  - sin datos suficientes (< 7 días con actividad) → `completar_perfil` o `ninguna`.
  - Implementado (`internal/consejos/reglas.go`): las reglas de estadísticas (franja, aspecto, reservas) piden ≥ 7 días activos en los últimos 28 (sin hoy); las de conteo (reseñas, cupón) siempre aplican. Pesos: reseñas 0.6 + 0.08 por reseña (hasta 5), franja 0.9 × déficit (≥ 40 % y ≥ 20 llegadas), aspecto 0.5 + caída (≥ 0.3 puntos y ≥ 3 reseñas por periodo, tope 1), cupón 0.55, reservas 0.5 (≥ 60 vistas en 28 días), perfil 0.2 (sin descripción o sin fotos). Una franja solo compite si ese bloque tuvo llegadas otro día y ese día tuvo llegadas (así no compara contra horas cerradas). El aspecto compara los últimos 28 días contra los 28 anteriores. Las colecciones de nv-61, nv-62 y nv-74 se leen por nombre, como hace nv-74.
- **Redacción:** con IA si hay cupo; si no, plantilla por regla. A Gemini solo va el hecho elegido y su contexto agregado (categoría, día y franja, porcentajes, conteos, nombre del aspecto), en `Contexto`; `TextoUsuario` va vacío. Nunca el nombre del negocio, de clientes, ids ni textos de reseñas.
- **Esquema de salida (nv-26):** `{ texto: string 20–160, accion: enum }`; la `accion` debe ser la del hecho (si el modelo la cambia o falla el esquema tras el reintento de `GenerarValidado`, se usa la plantilla). Instrucciones fijas y versionadas: tono amable, tuteo, sin prometer cifras que no estén en el contexto.
- **Respuesta:** `{ texto, accion, fuente: "ia" | "reglas", dia }`. `accion` ∈ `crear_cupon`, `ver_resenas`, `responder_resenas`, `editar_cupon`, `configurar_reservas`, `completar_perfil`, `ninguna`; la app la traduce a una pantalla.
- **Uno al día, sin Redis:** colección `consejos_negocio` (`negocio_id`, `dia` local, `texto`, `accion`, `fuente`, `regla`, `estado` `generando|listo`, `creado_en`, `expira_en` +30 días). Índices `ux_consejo_dia` `{negocio_id: 1, dia: 1}` único y `ttl_consejos` `{expira_en: 1}`. La primera petición del día inserta el documento en `generando` (el índice único decide quién genera); la que pierde la carrera responde la plantilla sin guardarla. Un documento en `generando` de hace más de 1 min se reintenta. Se genera al pedirlo, no con un trabajo nocturno para todos: así solo gasta cuota el negocio que entra.
- **Cupo propio:** `IA_CONSEJO_DIA` (nueva, por omisión 60, rango 0–200) consejos con IA por día para toda la app, contados en MongoDB (`consejos_negocio` con `fuente: ia` y `dia` de hoy). Al llegar, plantilla. Deja al menos 340 llamadas de las 400 para planes. 0 apaga la IA del consejo.
- **Límite por usuario:** `ia.ConUsuario(dueño)` cuenta en `IA_LIMITE_HORA`; un 429 de ese límite aquí no se propaga: se responde la plantilla (el consejo nunca falla por cuota).

### Descripción con IA
- **Body:** `palabras` exactamente 3, cada una de 2 a 30 caracteres, letras (con acentos), números, espacio o guion; sin URL ni correo. Si no, 422.
- **Contexto:** categoría, rango de precio y las 3 palabras en `TextoUsuario` (pasan por el anonimizado de nv-28). Sin dirección, teléfono ni datos del dueño.
- **Esquema:** `{ descripcion: string 60–300 }`; se recorta a 300 por runa y se quitan saltos de línea. No se guarda: la app la muestra y el dueño la usa en `PATCH /negocios/{id}` (que ya modera y valida `max=300`) o en el alta.
- **Respaldo sin IA:** plantilla «{categoría} con {p1}, {p2} y {p3}. …» con `fuente: "plantilla"`, para tope de gasto, límite global, Gemini caído, bloqueo o esquema inválido. Límite del usuario (`IA_LIMITE_HORA`) = 429 con `Retry-After` (como `sugerir-respuesta` de nv-62): repetir sin fin no es un uso válido.
- **Respuesta 200:** `{ descripcion, fuente: "ia" | "plantilla" }`.
- **Caché:** la de la guardia (nv-27, huella del prompt); las mismas 3 palabras en el mismo negocio no vuelven a llamar a Gemini.

### Paquete
- `internal/consejos`: reglas, service, handler, DTO, modelo y repositorio de `consejos_negocio`. La descripción va en `internal/negocios/panel` (junto al perfil que la edita), con su handler. Ambos reciben `*ia.Guardia` por constructor; sin guardia (`GEMINI_API_KEY` vacía) responden con reglas o plantilla.

### Privacidad
- `consejos_negocio` no tiene `usuario_id`; borra con la baja del negocio (`negocios.Observador.DadoDeBaja`). Logs: evento, `regla`, `fuente` y error; nunca el contexto mandado ni la salida.
- Prueba de privacidad: el `Contexto` y el `TextoUsuario` que recibe un `ia.Modelo` de prueba no contienen nombres, correos, ids de 24 hex ni frases de reseñas sembradas.

### Costos
- **Gemini:** consejo ~700 tokens de entrada y ~80 de salida; descripción ~400 y ~120. Con `IA_PRECIO_*` en 0 (capa gratuita) el gasto contado es 0; con facturación (precios de flash-lite), menos de USD 0.0002 por llamada, dentro de `IA_TOPE_USD_MES=5`. Peticiones: ≤ 60 al día por el consejo más las descripciones (acotadas por `IA_LIMITE_HORA`).
- **Redis:** consejo ya generado = lectura con sesión (política global 3 + revocación 1 = 4). Generación con IA = 4 + guardia 12 (caché 1, límite de usuario 3, gasto 1, límites globales 6 —3 por ventana—, guardar en caché 1) = 16; una vez por negocio y día. Descripción = escritura 7 + guardia 12 = 19 por llamada (un acierto de caché = 7 + 1). `TestPresupuestoRedis_ConsejoNV75` (en `tests/integration/consejos_test.go`) fija las tres cifras.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/negocios/{id}/consejo` | JWT dueño (comerciante) | 200, 401, 403, 404, 429, 503 |
| POST | `/api/v1/negocios/{id}/ia/descripcion` | JWT dueño (comerciante) | 200, 400, 401, 403, 404, 422, 429 |

Filas copiadas del Excel (A5): `GET /api/v1/negocios/{id}/consejo` «Consejo de la IA del día (US-142), p. ej. "Los martes de 4 a 6 tienes poca gente. ¿Lanzas un cupón?". Se genera una vez al día.» (—; 200 { texto, accion }; Dueño); `POST /api/v1/negocios/{id}/ia/descripcion` «"Que la IA me ayude": redacta la descripción a partir de 3 palabras.» (Body: palabras[3]; 200 { descripcion }; Dueño). Se copian a `docs/endpoints.md` al ejecutar.

**Drift con el Excel:** las respuestas agregan `fuente` (y `dia` en el consejo) para que la app sepa si vino de la IA. El 503 del consejo solo es MongoDB caído; la IA caída nunca da 503. La descripción durante el alta (negocio aún en borrador, nv-55) también es «Dueño».

## Tests
- `tests/integration/consejos_test.go`: cada regla con datos sembrados en `estadisticas_diarias`, `resenas` y `cupones`; segunda petición del día no llama al modelo; dos peticiones a la vez = una llamada; `IA_CONSEJO_DIA` agotado = `fuente: reglas`; modelo que cambia la acción o devuelve JSON inválido = plantilla; sin guardia = reglas.
- `tests/integration/negocios_descripcion_test.go`: 3 palabras = 200 `fuente: ia` con un `ia.Modelo` de prueba; 2 o 4 palabras = 422; Gemini caído = plantilla; límite del usuario = 429; caché por huella.
- `tests/security/consejos_security_test.go`: 401, 403 consumidor, dueño ajeno = 404; privacidad del contexto (sin nombres, correos, ids ni textos de reseñas); logs sin contexto ni salida.
- `TestPresupuestoRedis_ConsejoNV75` en `tests/integration/consejos_test.go` (no en `presupuesto_redis_test.go`, para no chocar con otras specs).
- `tests/database/indexes_test.go`: `ux_consejo_dia` y TTL. `tests/contract/consejos_contract_test.go`: 200 de ambas rutas (`testdata/consejos/`).
- Unit de la descripción en `internal/negocios/panel/descripcion_test.go`.
- Unit `internal/consejos`: cada regla y su peso, plantillas, esquema y validación de `palabras`.
- Prueba en vivo opcional con `GEMINI_API_KEY` (como `internal/ia/vivo_test.go`), fuera del CI.

## Criterios
- El consejo responde siempre y llama a Gemini a lo más una vez por negocio y día.
- Nada que identifique a un cliente llega al modelo.
- La IA del negocio usa a lo más `IA_CONSEJO_DIA` llamadas diarias y deja el resto a los planes.

## Límites
- Sin consejos por push ni historial de consejos para el dueño.
- Sin crear el cupón desde el consejo (la app abre la pantalla de nv-61).
- Sin traducción ni varias propuestas de descripción.

## Aprobación y Control de Cambios
- **Estado:** ✅ Aprobada (2026-11-19, Osmar Reséndiz): implementada y fusionada en `main` con el CI verde. Estado al firmar: Borrador con evidencia (2026-11-16), ola 12; implementada en la rama `claude/nv-75-a587c9`. Falta la firma.
- **Revisores:** Osmar Reséndiz (único integrante de backend)
- **Fecha:** aprobada 2026-11-19
- **Decisiones para el revisor (tomadas como las propone la spec, 2026-11-16):** reglas eligen el hecho y la IA solo lo redacta; consejo al pedirlo (no nocturno), uno por día en MongoDB con candado por índice único, sin Redis; variable nueva `IA_CONSEJO_DIA=60`; el consejo nunca da 429 ni 503 por IA; la descripción no se guarda, da 429 en el límite del usuario y plantilla en lo demás; respuestas con `fuente`.
- **Detalle:** evidencia en `specs/nv-75/checklists/gates.md` + `Memoria.md` al ejecutar.
