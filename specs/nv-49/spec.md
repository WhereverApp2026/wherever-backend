# NV-49 — Cifrar los datos sensibles en la base

## Contexto
Ánimo, alergias y fecha de nacimiento dejan de guardarse en claro, con migración segura. **Origen**: US-226. **Depende de**: NV-48.

## Requisitos
1. Lectura y escritura transparentes para el resto del código mediante una capa en el repositorio.
2. Migración idempotente de los datos que ya estén en claro.
3. El exportador de `nv-05` entrega los datos descifrados al dueño; el purgador los borra.
4. Nada sensible en claro en logs, errores ni copias de seguridad de pruebas.

## Diseño
- **Búsquedas:** estos campos no se buscan ni filtran por valor; el motor de nv-23 los lee ya descifrados.
- **Edad:** la fecha de nacimiento se cifra; la mayoría de edad se valida al registrar y se guarda el booleano.
- **Migración:** `cmd/migrar-cifrado`, con modo de prueba (solo cuenta) y modo real.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Repositorio 8` con MongoDB real: guardar y leer ánimo, alergias y fecha; el dato en la base no es legible; migración dos veces.

## Criterios
- En la base no hay ánimo, alergias ni edades en claro.
- El resto del código no cambia.

## Límites
- Sin cifrado de nombre ni correo (no son sensibles según la especificación).

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-49/checklists/gates.md` + `Memoria.md`.
