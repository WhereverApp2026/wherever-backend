# NV-34 — Nombre de usuario y privacidad de búsqueda

## Contexto
La búsqueda de personas solo funciona con quien lo permite; el teléfono es apagado por defecto. **Origen**: US-232, US-233, US-236 y hoja J2 del Excel. **Depende de**: NV-05.

## Requisitos
1. Campo nuevo `nombre_usuario` (3 a 20 caracteres, minúsculas, números y guion bajo; único sin importar mayúsculas), que la persona define con `PATCH /usuarios/yo`.
2. Ajuste nuevo `buscable_por_usuario` (encendido por defecto) en `PATCH /usuarios/yo/privacidad`. `buscable_por_telefono` ya existe (nv-05, apagado por defecto) y se reutiliza. Ambos con efecto inmediato.

## Diseño
- **Formato:** `^[a-z0-9_]{3,20}$` con la regla `username` de `internal/shared/validation`. Las mayúsculas son 422 (no se convierten): así el índice único parcial `ux_nombre_usuario` ya es único sin importar mayúsculas. Repetido = 409.
- **Reservados:** `admin`, `administrador`, `wherever`, `soporte`, `ayuda`, `root`, `sistema`, `moderador`, `oficial`, `api`, `null` (coincidencia exacta).
- **Por defecto:** `privacidad.buscable_por_usuario` es `*bool`; ausente (cuentas previas) cuenta como encendido. nv-35 filtra con `{$ne: false}`.
- `GET /usuarios/yo` devuelve `nombre_usuario` (`null` si no lo ha elegido); entra en la exportación ARCO con el perfil.

## Contratos
Sin endpoints nuevos. Cambian los cuerpos de `PATCH /api/v1/usuarios/yo` (+`nombre_usuario`, 409 si se repite) y `PATCH /api/v1/usuarios/yo/privacidad` (+`buscable_por_usuario`).

## Tests
- `Unit/NombreUsuario 6`: formato, longitud, mayúsculas, reservados (`internal/shared/validation/username_test.go`); `TestPrivacy_SearchableByUsernameDefaultsOn` (`internal/usuarios`).
- `Integration`: `TestUsuariosService_UsernameUniqueAndSearchableToggle` (409 al repetir, apagado inmediato).
- `Database`: `ux_nombre_usuario` único. `Contract`: perfiles, privacidad y exportación con las llaves nuevas.

## Criterios
- El teléfono nunca sale de la API.
- Quien no activó la búsqueda por teléfono no aparece por ese medio (lo prueba la búsqueda de nv-35).

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada (2026-11-11), Borrador con evidencia; ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-34/checklists/gates.md` + `Memoria.md`.
