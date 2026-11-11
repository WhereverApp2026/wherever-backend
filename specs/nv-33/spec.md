# NV-33 — Respaldo, cancelación y «otra idea»

## Contexto
Si la IA falla o tarda, la persona siempre recibe opciones; también puede pedir otro plan o cancelar. **Origen**: US-035, US-039, US-040, US-041, US-042, US-043 y hoja A4 del Excel. **Depende de**: NV-32 (flujo de dos pasos) y NV-27/NV-28 (`ia.Guardia`: tope, caché y límites).

## Requisitos
1. **Respaldo (US-039):** si la IA tarda más del plazo, falla, el proveedor responde 429, la respuesta es inválida o bloqueada, se agotó el tope, el límite global o el del usuario, o no hay proveedor configurado, responde el motor de reglas con un plan de la misma forma que nv-31, armado con sus mejores candidatos, `fuente: reglas` y un aviso.
2. **Cancelar (US-040):** si la petición se cancela, la API cancela la llamada al modelo y no arma el respaldo.
3. `POST /planes/generar/otra-idea` produce un plan distinto excluyendo los lugares ya propuestos.
4. Respuesta promedio por debajo de 5 s con la IA y por debajo de 1 s con el respaldo.

## Diseño
- **Plazo total de 8 s:** las dos llamadas a la IA comparten un contexto de 7 s (`asistente.PlazoTotal` menos 1 s de reserva); el segundo restante es para el motor de reglas, que tarda milisegundos.
- **Respaldo secuencial, no en paralelo** (corrección de drift): el motor corre después de la falla y reutiliza lo que la IA ya entendió (si el paso 1 respondió y falló el 2, el respaldo conserva palabras clave, compañía y presupuesto). Correrlo desde el inicio duplicaba la consulta al catálogo en cada petición sin ganar tiempo dentro del plazo.
- **Sin interpretación** (falla en el paso 1 o IA apagada): `compania: sin_dato`, sin palabras clave; el motor usa el perfil (gustos, alergias, dieta, presupuesto) y la hora pedida o ahora.
- **Guardia:** `cmd/api` y `authtest` envuelven el modelo en `ia.Guardia`; el asistente llama a `Guardia.GenerarValidado` (caché y límite del usuario) y pasa `ia.ConUsuario`. Un plan con IA son 2 generaciones, por eso `IA_LIMITE_HORA` pasa de 5 a 10 (5 planes por hora).
- **Límite y costo:** el respaldo no llama al modelo: no suma gasto ni cuenta en los límites. Las llamadas rechazadas por tope o límite tampoco cuestan.
- **Cancelación:** el contexto de la petición (plazo de 60 s del middleware o apagado) cancela la llamada. Fiber (fasthttp) no avisa cuando la app corta la conexión; el plazo de 7 s acota lo que se gasta en ese caso.
- **Otra idea:** el cuerpo de `generar` más `excluir` (1 a 30 ids). Los ids van a `recomendador.Contexto.Excluidos`, que ya filtra en MongoDB (`$nin`) y en Go. Sin más lugares: 422.
- **Errores:** ya no hay 429 ni 503 por la IA; 429 queda para el rate limit global y 503 para catálogo o perfil caídos.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/generar` | JWT | 200, 400, 401, 403, 422, 429, 503 |
| POST | `/api/v1/planes/generar/otra-idea` | JWT | 200, 400, 401, 403, 422, 429, 503 |

## Tests
- Unit `internal/asistente/respaldo_test.go`: cada falla de la IA (tiempo, 429, límite del proveedor, falla, bloqueada, inválida, IA apagada) termina en reglas con 3 paradas sin el alérgeno; tope, límite del usuario y global con una `ia.Guardia` de precio positivo no llaman al proveedor ni suman gasto; la IA lenta cae a reglas en menos de 1 s; la cancelación corta la IA sin respaldo; otra idea no repite y sin lugares da 422; validación de `excluir`; motivo por error.
- `Integration/Asistente` (`tests/integration/asistente_test.go`) con MongoDB y Redis reales y el doble de `ia.Modelo` detrás de la guardia real: plan correcto, la IA inventa un lugar, la IA tarda más del plazo, 429, falla, inválida, itinerario vacío, límite del usuario, tope agotado (sin sumar gasto), sin candidatos, frase de 301; `OtraIdea`: no repite, nunca trae un negocio no aprobado, sin más lugares 422.
- `Security/Asistente`: 401 sin sesión, token vencido o usuario inexistente y 403 con eliminación pendiente, en las dos rutas. La inyección de instrucciones la cubre `Security/IA` (nv-28).
- Medición con IA real (pendiente, necesita la llave de SSM): 20 frases en español, tasa de descartes, latencia p50 y p95, tasa de 429 y tokens por plan.

## Criterios
- Con la IA caída, el usuario siempre recibe 3 opciones (si hay 3 lugares compatibles).
- La tasa de descartes queda medida (`ia_descartes`, nv-32).

## Límites
- La calidad depende del modelo; se mide, no se promete.
- Los traslados son estimados (nv-20).
- Sin memoria de conversación: cada petición es independiente; la app manda `excluir`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11): implementada en la rama `nv-33`; falta la medición con IA real y la firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** respaldo secuencial en lugar de paralelo; `IA_LIMITE_HORA` 10; el límite del usuario ya no da 429 sino respaldo.
- **Detalle:** evidencia en `specs/nv-33/checklists/gates.md` + `Memoria.md`.