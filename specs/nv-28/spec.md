# NV-28 — Límite por usuario y protección de datos en la IA

## Contexto
Un usuario no puede agotar la IA y nada personal viaja al modelo. **Origen**: US-159, US-190. **Depende de**: NV-27.

## Requisitos
1. **Límite por usuario** y por hora (`IA_LIMITE_HORA`, 10 por defecto) con `Retry-After` y la hora exacta de regreso; el uso queda registrado.
2. **Defensa contra inyección:** el texto del usuario va solo en el turno de usuario, nunca en las instrucciones; el modelo solo puede elegir ids de una lista que la API le entrega.
3. **Sin datos personales hacia el modelo:** sin nombre, correo, teléfono ni coordenadas exactas (se envían distancias y zona aproximada).

## Diseño
- **Redis:** el límite por usuario sigue cerrado ante fallos; la caché no.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Límite 6` con Redis real: décima y undécima petición, hora de regreso, otro usuario no se ve afectado, Redis caído (cierra).
- `Security/IA 4`: texto con instrucciones («ignora lo anterior») no cambia las reglas; el texto del usuario nunca aparece en las instrucciones.

## Criterios
- Ninguna llamada lleva datos personales.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-28/checklists/gates.md` + `Memoria.md`.
