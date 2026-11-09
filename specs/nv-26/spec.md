# NV-26 — Cliente de Bedrock y salida validada

## Contexto
La IA generativa corre en Amazon Bedrock con el rol de la instancia, sin llaves, y su salida siempre se valida. **Origen**: US-159, US-190. **Depende de**: NV-11, NV-12.

## Requisitos
1. Interfaz `ia.Modelo` con un adaptador de **Amazon Bedrock** (API Converse) que usa el rol de la instancia.
2. **Prueba previa de la organización y de la cuenta:** una llamada de prueba desde la instancia por Systems Manager confirma que ninguna política de la organización bloquea Bedrock y que la cuenta, en el plan gratuito, puede usar el modelo elegido. Los modelos de terceros se contratan por AWS Marketplace, que el plan gratuito puede no permitir; se prefiere un modelo de Amazon (Nova). Es lo primero que se ejecuta de la ruta: si falla, se decide antes de seguir.
3. **Salida validada:** el modelo responde JSON con esquema; una respuesta que no valida se descarta (un reintento) y cuenta como fallo.
4. Permisos mínimos: `bedrock:InvokeModel` solo sobre el modelo elegido en el rol de la instancia y en el boundary.

## Diseño
- **Paquete** `internal/ia`: `modelo.go` (interfaz), `bedrock.go`, `gasto.go`, `cache.go`, `limite.go`, `esquema.go`.
- **Tiempos:** `IA_TIMEOUT` de 25 s por llamada; el asistente aplica además su plazo de 8 s (nv-31).
- **Pruebas:** el adaptador se prueba contra un servidor local que habla el protocolo de Converse; una prueba en vivo (`IA_LIVE=1`) corre una vez antes del congelamiento.
- **Permiso para el JSON de IAM:** `bedrock:InvokeModel` y, si el modelo usa perfil de inferencia entre regiones, sobre el perfil y los modelos de esas regiones.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Esquema 6`: salida válida, campo faltante, id fuera de la lista, texto extra, JSON truncado.
- `Integration/Bedrock 6` contra servidor local de Converse: éxito, lento (tiempo), 429 del servicio, error 5xx, respuesta inválida con reintento.
- En vivo, una vez: una llamada real con el modelo elegido; se registra costo y latencia.

## Criterios
- Cumple los requisitos de arriba.

## Límites
- Bedrock se paga con los créditos: el tope de nv-27 debe caber en lo que queda después del gasto de la infraestructura.
- El costo se estima; la factura real manda.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-26/checklists/gates.md` + `Memoria.md`.
