# NV-27 — Tope de gasto y caché de la IA

## Contexto
El gasto de la IA tiene un tope mensual y las respuestas repetidas se reutilizan. **Origen**: US-159, US-190. **Depende de**: NV-26.

## Requisitos
1. **Tope de gasto mensual** (`IA_TOPE_USD_MES`, 5 por defecto, mayor que 0 y hasta 50): se estima el costo por llamada con los tokens reales; al llegar al tope, las llamadas devuelven `ErrTope` y el llamador usa el motor de reglas. En la capa gratuita de Gemini el costo es cero y el tope no se alcanza; protege si se activa la facturación, con el precio en `IA_PRECIO_ENTRADA_USD_MTOK` e `IA_PRECIO_SALIDA_USD_MTOK` (USD por millón de tokens, 0 por defecto).
2. **Caché** de respuestas por huella de la entrada normalizada, 1 hora de vida.

## Diseño
- **Pieza:** `ia.Guardia` (`internal/ia/guardia.go`) envuelve un `ia.Modelo` y es a su vez un `Modelo`: cada llamada al proveedor pasa por el tope, incluido el reintento de `ia.GenerarValidado`. `Guardia.GenerarValidado` agrega la caché. Redis y MongoDB en `internal/ia/almacen.go`.
- **Gasto:** contador en micro-USD en `cuota:ia_gasto:{AAAA-MM}` (mes en UTC; vive hasta un día después del mes) y registro en la colección `usos_ia` (`usuario_id`, modelo, mes, tokens de entrada y salida, `costo_micro_usd`, resultado, `duracion_ms`; sin la frase ni la salida). Si falta la llave, se reconstruye sumando `usos_ia` del mes (`SET NX`); si Redis falla, se usa `usos_ia` directo; si fallan los dos, `ErrGastoDesconocido` y no se llama al proveedor.
- **Caché:** `cache:ia:{huella}`, 1 hora. La huella es el SHA-256 de modelo, instrucciones, contexto (la lista de candidatos), frase normalizada (anonimizada, minúsculas, espacios colapsados), esquema y tope de salida. Solo se guardan salidas ya validadas, y nunca una que repita la frase del usuario. La caché no cierra: si Redis falla, se llama al proveedor con el tope de `usos_ia`.
- **Privacidad:** `usos_ia` se registra en ARCO (`cmd/api` y `authtest`). Borrar el historial o la cuenta quita `usuario_id` y conserva el costo para el tope; la exportación lista los usos sin ids. Los registros viven 400 días (TTL `ttl_uso`).
- **Presupuesto de Redis:** por llamada al proveedor, gasto 2 (`GET` e `INCRBY`; con costo cero solo el `GET`); por generación, caché 2 (`GET` y `SET`; 1 si acierta). Con 50 generaciones al día son menos del 2 % de la cuota de Upstash.
- **Sin cableado todavía:** ningún endpoint llama a la IA; quien la use (nv-31, nv-33) arma la guardia con `ia.NewGuardia` y la configuración de `config.IA`.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Gasto 8` (`internal/ia/guardia_test.go`): costo por tokens, suma por llamada, tope exacto, mes nuevo, reconstrucción desde `usos_ia`, Redis caído, sin gasto conocido cierra y tokens de una salida inválida; más la huella y la caché con dobles en memoria. Configuración en `internal/platform/config/config_test.go`.
- `Integration/Caché 4` (`tests/integration/ia_guardia_test.go`): misma entrada acierta, entrada distinta falla, el valor guardado no contiene el texto del usuario y vive 1 hora, y una salida que repite la frase no se guarda. Además el tope con Redis y MongoDB reales (no llama y se reconstruye) y la privacidad de `usos_ia` (desliga y exporta).

## Criterios
- La IA no se llama si se agotó el tope o el límite del usuario.
- El costo estimado coincide con la factura dentro de un margen razonable.

## Límites
- La caché guarda poco: casi todas las peticiones llevan contexto de usuario.
- Dos llamadas simultáneas pueden pasar el tope por el costo de una: el costo se conoce después de la llamada.
- El mes es UTC; la factura de Google usa la hora del Pacífico, así que el corte difiere unas 8 horas.
- El tope es mensual de toda la aplicación, no por usuario.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada (2026-11-11), Unit verde; Integration en el CI del PR; falta la firma
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-11
- **Decisiones para el revisor:** colección `usos_ia` (no `ia_uso`, por la regla de nombres en plural); llaves `cuota:ia_gasto:{mes}` y `cache:ia:{huella}`; mes en UTC; tope máximo configurable de 50 USD (el presupuesto de la cuenta); ARCO desliga en lugar de borrar para no bajar el gasto del mes.
- **Detalle:** la evidencia irá en `specs/nv-27/checklists/gates.md` + `Memoria.md`.
