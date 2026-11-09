# NV-27 — Tope de gasto y caché de la IA

## Contexto
El gasto de la IA tiene un tope mensual y las respuestas repetidas se reutilizan. **Origen**: US-159, US-190. **Depende de**: NV-26.

## Requisitos
1. **Tope de gasto mensual** (`IA_TOPE_USD_MES`, 5 por defecto): se estima el costo por llamada con los tokens reales; al llegar al tope, las llamadas devuelven `ErrTope` y el llamador usa el motor de reglas.
2. **Caché** de respuestas por huella de la entrada normalizada, 1 hora de vida.

## Diseño
- **Gasto:** contador mensual en Redis y registro en la colección `ia_uso` (usuario, tokens entrada y salida, costo estimado, resultado, duración).
- **Caché:** la clave es la huella (SHA-256) de modelo, instrucciones, entrada normalizada y lista de candidatos.
- **Presupuesto de Redis:** cada generación suma unos 4 comandos (caché 2, gasto 2) a los del límite de nv-28. Con 50 generaciones al día son menos del 2 % de la cuota de Upstash.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Gasto 8`: suma por llamada, tope exacto, mes nuevo, reconstrucción desde `ia_uso`.
- `Integration/Caché 4`: misma entrada acierta, entrada distinta falla, el valor guardado no contiene el texto del usuario.

## Criterios
- La IA no se llama si se agotó el tope o el límite del usuario.
- El costo estimado coincide con la factura dentro de un margen razonable.

## Límites
- La caché guarda poco: casi todas las peticiones llevan contexto de usuario.
- El tope es mensual de toda la aplicación, no por usuario.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-27/checklists/gates.md` + `Memoria.md`.
