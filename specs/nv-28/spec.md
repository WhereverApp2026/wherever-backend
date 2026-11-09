# NV-28 — Límite por usuario y protección de datos en la IA

## Contexto
Un usuario no puede agotar la IA y nada personal viaja al modelo. **Origen**: US-159, US-190. **Depende de**: NV-27.

## Requisitos
1. **Límite por usuario** y por hora (`IA_LIMITE_HORA`, 5 generaciones por defecto, unas 10 llamadas) con `Retry-After` y la hora exacta de regreso; el uso queda registrado.
4. **Límite global por debajo de la capa gratuita de Gemini:** `IA_LIMITE_MIN` (llamadas por minuto, 5 por defecto) e `IA_LIMITE_DIA` (llamadas por día, 16 por defecto), compartidos por las dos instancias en Redis. Cada uno se fija en no más del 80 % del límite que AI Studio muestra para el proyecto y el modelo; el día se cuenta en hora del Pacífico, como el del proveedor. Al llegar a cualquiera de los dos, responde el motor de reglas sin llamar al proveedor.
2. **Defensa contra inyección:** el texto del usuario va solo en el turno de usuario, nunca en las instrucciones; el modelo solo puede elegir ids de una lista que la API le entrega.
3. **Sin datos personales hacia el modelo:** sin nombre, correo, teléfono ni coordenadas exactas (se envían distancias y zona aproximada). La frase pasa por `ia.Anonimizar` (nv-26) y nunca viajan ids de usuario. La capa gratuita de Gemini puede usar lo enviado para mejorar sus productos.

## Diseño
- **Redis:** el límite por usuario y el global siguen cerrados ante fallos (sin Redis no se llama al proveedor y responde el motor de reglas); la caché no.
- **Llaves:** `rate:ia_usuario:{usuario}` (1 h), `rate:ia_min:{minuto}` (1 min) y `rate:ia_dia:{fecha_pacifico}` (25 h).
- **Presupuesto de Redis:** el límite global suma unos 4 comandos por generación; sigue por debajo del 2 % de la cuota con 50 generaciones al día.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Límite 8` con Redis real: quinta y sexta generación, hora de regreso, otro usuario no se ve afectado, límite global por minuto y por día, Redis caído (cierra).
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
