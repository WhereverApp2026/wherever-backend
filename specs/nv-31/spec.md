# NV-31 — Interpretar la frase y armar candidatos

## Contexto
La IA convierte una frase libre en parámetros y la API elige los candidatos reales. **Origen**: US-035, US-039, US-040, US-041, US-042, US-043 y hoja A4 del Excel. **Depende de**: NV-20, NV-23, NV-26, NV-29.

## Requisitos
1. `POST /planes/generar` recibe una frase de hasta 300 caracteres y devuelve un **plan sin guardar** (la app lo guarda con `POST /planes`, nv-30), con `fuente` (`ia` o `reglas`), `aviso` y los `parametros` entendidos. El respaldo de nv-33 responde con esta misma forma.
2. **Interpretar (US-041):** la IA convierte la frase en un JSON validado: compañía (`solo`, `pareja`, `amigos`, `familia`, `sin_dato`), con niños, con mascota, presupuesto por persona, hora de inicio, ánimo y hasta 5 palabras clave.
3. **Candidatos:** el motor de reglas (nv-23, `recomendador.Recomendar`) arma una lista de 15 negocios abiertos a la hora de inicio y compatibles con alergias y dieta del perfil; de ellos sale el plan.
4. **Armado por reglas (nv-31):** hasta 3 paradas de los candidatos en el orden del motor, primero de giros distintos, cada una abierta al llegar y al salir y dentro del presupuesto por persona; `planes.Recalcular` (nv-29) escribe traslados y costos.

## Diseño
- **Flujo:** frase → `Interpretar` (IA) → parámetros + perfil → `Recomendar` (15 candidatos) → `Armar` → `Recalcular` → respuesta. El itinerario con IA (`armarItinerario`, solo ids candidatos) y su verificación son de nv-32; el respaldo, el plazo de 8 s y la cancelación, de nv-33.
- **Paquete `internal/asistente`:** `Interpretar`, `Armar`, `Service.Generar` y el handler. Depende solo de la interfaz `ia.Modelo`; el armado con `ia.Guardia` (tope, caché y límites de nv-27/nv-28) entra con nv-33, cuando esas ramas estén en la base. Sin colección ni índices nuevos.
- **Instrucciones fijas** en `internal/asistente/prompts/interpretar.txt` (embebido); la frase va solo en `TextoUsuario`. Se valida con `ia.GenerarValidado` (un reintento) y `Interpretacion.Validar` (catálogos, presupuesto 0–20 000, hora `HH:MM`, palabras ≤ 30 caracteres).
- **Datos hacia el modelo:** las instrucciones, el día y la hora local y la frase anonimizada (`ia.Anonimizar`, también dentro del adaptador). Nunca ids, nombre, correo, ubicación, gustos ni alergias del perfil: el proveedor es Gemini en capa gratuita (nv-26). Compañía, presupuesto, hora y etiquetas salen del modelo, no entran a él.
- **Perfil:** alergias (`gustos.alergias`) y dieta (`gustos.restricciones`) son filtro duro siempre; los gustos se suman a las palabras clave; sin monto en la frase manda `gustos.presupuesto_min/max`. El ánimo se devuelve solo con `privacidad.estado_animo`.
- **Inicio:** `fecha` del cuerpo (de ahora a 30 días); si no viene, la hora de la frase hoy (mañana si ya pasó); si no, ahora redondeado a 5 min. Hora de `America/Mexico_City`. Estancia por giro: restaurante 90, cafetería 60, bar 90, entretenimiento 120, servicios 60, comercio 45 min.
- **Personas y movilidad:** las del cuerpo; si no, `pareja` = 2 y lo demás 1; movilidad de `preferencias.movilidad_default` o `a_pie`. `presupuesto` del plan = presupuesto por persona × personas.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/generar` | JWT | 200, 400, 401, 403, 422, 429, 503 |

- 422: cuerpo inválido o ningún candidato cabe. 429: el proveedor no tiene cupo (`Retry-After`). 503: IA apagada (sin `GEMINI_API_KEY`), lenta, bloqueada o con respuesta inválida tras el reintento. 403: cuenta con eliminación pendiente.

## Tests
- `Unit/Interpretar 6` (`internal/asistente/interpretar_test.go`): frase completa, frase corta («relax»), sin presupuesto (manda el perfil), texto con instrucciones, frase anonimizada sin datos personales, errores del proveedor y respuestas inválidas.
- `Unit/Armar` (`armar_test.go`): giros distintos y orden horario, presupuesto, negocio que cierra antes de terminar, sin candidatos, alergias y dieta siempre como filtro duro.
- `Unit/Service` (`service_test.go`): validación del cuerpo, `fecha` e inicio, 429 y 503 sin texto del proveedor en logs.
- `Integration/Asistente 3` (`tests/integration/asistente_test.go`): caso feliz con candidatos reales (alergia excluida, plan sin guardar, solo la frase anonimizada llega al modelo), ánimo con consentimiento, 429/503/422.
- `Security/Asistente` (`tests/security/asistente_security_test.go`): 401 sin token, vencido o `sub` inexistente; 403 con eliminación pendiente; nada llega al modelo.

## Criterios
- Cumple los requisitos de arriba con el proveedor sustituido por un doble de `ia.Modelo` (`internal/testutil/iatest`).
- Ninguna petición al modelo contiene id, correo, nombre, ubicación ni alergias de la persona.

## Límites
- Sin política de rate limit propia: los límites de IA por usuario y globales viven en `ia.Guardia` (nv-28) y se conectan con nv-33. **No fusionar a `main` sin nv-33** o sin un límite equivalente en la ruta.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada (2026-11-11) en la rama `nv-31` sobre `nv-29`; falta la firma y el CI del PR. Ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** armado por reglas en nv-31 (el itinerario con IA pasa a nv-32); IA apagada = 503 hasta nv-33; estancias por giro; `presupuesto` del plan = por persona × personas.
- **Detalle:** evidencia en `specs/nv-31/checklists/gates.md` + `Memoria.md`.
