# NV-34 — Nombre de usuario y privacidad de búsqueda

## Contexto
La búsqueda de personas solo funciona con quien lo permite; el teléfono es apagado por defecto. **Origen**: US-232, US-233, US-236 y hoja J2 del Excel. **Depende de**: NV-05.

## Requisitos
1. Campo nuevo `nombre_usuario` (3 a 20 caracteres, minúsculas, números y guion bajo; único sin importar mayúsculas), que la persona define con `PATCH /usuarios/yo`.
2. Ajuste nuevo `buscable_por_usuario` (encendido por defecto) en `PATCH /usuarios/yo/privacidad`. `buscable_por_telefono` ya existe (nv-05, apagado por defecto) y se reutiliza. Ambos con efecto inmediato.

## Diseño
- Sin decisiones de diseño propias: sigue lo definido arriba.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/NombreUsuario 6`: formato, longitud, mayúsculas, reservados.

## Criterios
- El teléfono nunca sale de la API.
- Quien no activó la búsqueda por teléfono no aparece por ese medio.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-34/checklists/gates.md` + `Memoria.md`.
