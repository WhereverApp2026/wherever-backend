# NV-25 — Clima e Inicio

## Contexto
La pantalla de Inicio se arma en una sola llamada con saludo, clima, chips por hora y recomendaciones. **Origen**: US-166, US-025, US-037 y hoja A2 del Excel. **Depende de**: NV-24.

## Requisitos
1. `GET /inicio` arma la pantalla en una llamada: saludo por hora, clima, chips según franja (mañana, tarde, noche) y 5 recomendaciones.
2. `GET /contexto/clima` devuelve el clima de la ciudad o coordenada, con caché de 30 minutos.

## Diseño
- **Clima (`internal/clima`):** Open-Meteo (`/v1/forecast?current=temperature_2m,weather_code,is_day,precipitation`), sin llave, plazo de 3 s. Caché **en memoria por instancia** de 30 min por celda de 0,01° (~1 km); un fallo se recuerda 1 min para no esperar 3 s en cada Inicio. Sin Redis: el clima no depende del usuario y así no gasta cuota de Upstash. A Open-Meteo solo sale la coordenada redondeada; los logs no llevan coordenadas.
- **Factor clima (`internal/recomendador`):** `Contexto.ClimaAdverso` (nil = sin dato, neutro 0,5 como antes). Mal clima = lluvia, llovizna, nieve, tormenta, precipitación ≥ 0,2 mm, ≥ 32 °C o ≤ 8 °C. Con mal clima el aire libre (servicio o etiqueta `terraza`, `jardin`, `patio`, `aire libre`) vale 0 y bajo techo 1; con buen clima el aire libre vale 1 y el resto 0,5. Mismo peso (0,08).
- **Inicio (`internal/inicio`):** sesión opcional. Franja en `America/Mexico_City`: mañana 05:00–11:59, tarde 12:00–18:59, noche el resto; saludo y 3 chips por franja sobre las categorías de nv-20. Cinco recomendaciones con `recomendaciones.Service.ForHome`: sin sesión (o cuenta no activa) son generales, sin gustos, alergias ni descartes.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/inicio?lat=&lng=&compania=&hora=` | opcional (token inválido = 401) | 200, 401, 422, 503 |
| GET | `/api/v1/contexto/clima?lat=&lng=` o `?ciudad_id=` | no | 200 (sin servicio `{disponible: false}`), 422 |

## Tests
- `Unit/Clima` (`internal/clima/clima_test.go`): respuesta normal, servicio lento, servicio caído (se recuerda 1 min), caché de 30 min por celda, códigos WMO y mal clima. Open-Meteo se imita con `internal/testutil/climatest` (interfaz externa).
- `Unit/Inicio` (`internal/inicio/inicio_test.go`) y `Unit/Puntaje_Clima` (`internal/recomendador/puntaje_test.go`).
- `Integration/Clima 3` e `Integration/Inicio 4` (`tests/integration/inicio_test.go`): normal, por ciudad y caché; lento (Inicio omite el factor) y caído; validación; saludo y chips por franja con hora fija; sin sesión recibe lo general; clima caído; token inválido.

## Criterios
- Cumple los requisitos de arriba; con Open-Meteo lento o caído Inicio responde 200 con `clima.disponible: false` y el orden de antes de nv-25.

## Límites
- El clima depende de un servicio gratuito sin garantía (uso no comercial, unas 10 000 llamadas al día). Con la caché, cada instancia pide cada celda a lo más una vez cada 30 min.
- Las recomendaciones de `GET /recomendaciones` (nv-24) siguen sin clima; solo Inicio lo usa.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-12): implementada; local en verde (build, vet, Unit, lint, critic, SDD, endpoints). Integration, Security, Database y Contract en el CI del PR.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** caché en memoria en lugar de Redis; franjas, chips y umbrales de mal clima propuestos arriba.
- **Detalle:** evidencia en `specs/nv-25/checklists/gates.md` + `Memoria.md`.
