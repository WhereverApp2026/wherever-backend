# NV-26 — Cliente de IA (Gemini) y salida validada

## Contexto
La IA generativa corre en la API de Gemini, capa gratuita, detrás de la interfaz `ia.Modelo`, y su salida siempre se valida. Bedrock se descartó: la cuenta de AWS tiene cuota cero para los modelos (2026-11-09). **Origen**: US-159, US-190. **Depende de**: NV-11, NV-12.

## Requisitos
1. Interfaz `ia.Modelo` con un adaptador de la **API REST de Gemini** (`generateContent`) que pide salida JSON con esquema (`responseMimeType` y `responseJsonSchema`).
2. **Llave:** `GEMINI_API_KEY` sale de SSM (`/wherever/dev/GEMINI_API_KEY`, SecureString) o de `.env`. Nunca en Git, en la imagen ni en los logs; viaja en el encabezado `x-goog-api-key`, nunca en la URL. El modelo va en `IA_MODELO_ID`. Sin llave, la IA queda apagada y responde el motor de reglas.
3. **Salida validada:** una respuesta que no es JSON, no cumple el esquema, trae texto extra o se cortó por tokens se descarta, con un reintento, y cuenta como fallo.
4. **429 del proveedor:** es `ErrLimiteProveedor`, no se reintenta y el llamador responde con el motor de reglas (nv-33). Nuestros límites (nv-28) quedan por debajo de los de la capa gratuita para que el 429 sea la excepción.
5. **Sin datos personales hacia el proveedor:** el adaptador anonimiza el texto del usuario (correos, teléfonos, enlaces, usuarios y coordenadas) y no envía ids de usuario. La capa gratuita de Gemini usa lo enviado para mejorar sus productos y personas pueden leerlo, así que solo viaja texto sin identificar.

## Diseño
- **Paquete** `internal/ia`: `modelo.go` (interfaz y errores), `gemini.go`, `esquema.go` (`Decodificar` y `GenerarValidado`), `anonimizar.go`. Gasto, caché y límites llegan con nv-27 y nv-28.
- **Petición:** instrucciones solo en `systemInstruction`; en el turno de usuario, primero el contexto que arma la API y después la frase anonimizada, en partes separadas. Temperatura 0,2.
- **Tiempos:** `IA_TIMEOUT` de 25 s por llamada (máximo 55 s); el asistente aplica además su plazo de 8 s (nv-31).
- **Errores:** 429 → `LimiteError` con la espera que sugiere el proveedor; plazo vencido, 408 o 504 → `ErrTiempo`; 5xx y otros 4xx → `ErrProveedor` con el código y el estado, sin el cuerpo; bloqueo por filtros → `ErrBloqueada`.
- **Red:** las instancias ya salen por 443 a cualquier destino (`main.tf`, `egress_ports.https`); `generativelanguage.googleapis.com` no necesita regla nueva.
- **Términos de la capa gratuita:** exigen 18 años o más; el registro ya lo exige (`adult`, nv-05). No se puede usar con personas del EEE, Suiza o Reino Unido; el piloto es en Tula.
- **Bedrock, opción futura:** si la cuenta obtiene cuota, un adaptador `bedrock.go` (API Converse con el rol de la instancia y `bedrock:InvokeModel` sobre el modelo elegido) implementa la misma interfaz. No se escribe ahora.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Esquema 6`: salida válida, campo faltante, id fuera de la lista, texto extra, JSON truncado, campo no declarado.
- `Unit/Gemini 9` contra un servidor local que habla `generateContent`: éxito con la petición separada y anonimizada, la llave solo en el encabezado, lento (tiempo), 429 sin reintento y con su espera, 5xx sin cuerpo, respuesta inválida con reintento, dos inválidas, bloqueo, configuración sin llave.
- `Unit/Anonimizar 7`: correo, teléfonos, enlace, usuario, coordenadas; los montos y las horas se conservan.
- En vivo, una vez (`IA_LIVE=1`): las 20 frases de `internal/ia/testdata/frases.txt` con la llave real; se registran latencia p50 y p95, tasa de 429, salidas inválidas y tokens.

## Criterios
- Cumple los requisitos de arriba.
- La llave no aparece en Git, en logs ni en mensajes de error.

## Límites
- La capa gratuita no tiene acuerdo de nivel de servicio; sus límites se ven por proyecto en AI Studio y pueden cambiar.
- Un nombre propio en la frase no se detecta; la app pide no escribir datos personales.
- El costo es cero en la capa gratuita; si se activa la facturación, el tope de nv-27 manda.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada y probada en vivo (20 de 20 válidas, sin 429, p95 1,2 s); falta la firma
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-09
- **Decisiones del revisor (2026-11-09):** Gemini en lugar de Bedrock por la cuota cero; misma interfaz; 429 al motor de reglas; límites propios por debajo de la capa gratuita; embeddings de nv-50 en Gemini; sin datos personales al proveedor; el aviso de privacidad nombra al proveedor externo.
- **Detalle:** la evidencia va en `specs/nv-26/checklists/gates.md` + `Memoria.md`.
