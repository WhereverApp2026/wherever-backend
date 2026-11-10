# NV-48 — Capa de cifrado de campos

## Contexto
Un paquete cifra y descifra campos sensibles con AES-256-GCM y llaves rotables. **Origen**: US-226. **Depende de**: NV-05.

## Requisitos
1. Cifrado por campo con AES-256-GCM para ánimo, alergias y fecha de nacimiento.
2. La llave vive en SSM como cadena segura (`/wherever/dev/DATA_KEYS`), con identificador de versión por llave para poder rotarla.
3. Cada valor guarda `enc:v1:<id_llave>:<nonce>:<texto cifrado>`; el nonce es aleatorio de 12 bytes y el dato asociado incluye colección, campo e id del documento para impedir copiar un valor a otro lugar.
4. Rotación: llaves nuevas cifran; las viejas siguen descifrando; un comando re-cifra todo.

## Diseño
- **Paquete** `internal/platform/cifrado`: `Cifrar`, `Descifrar`, llavero cargado al arrancar.
- **Fallo seguro:** si falta la llave, la API no arranca en producción; en desarrollo avisa.
- **Rendimiento:** el costo por valor es de microsegundos; se mide.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Cifrado 12`: ida y vuelta, nonce distinto cada vez, valor alterado (falla), valor copiado a otro documento (falla), llave equivocada, rotación.
- `Security/Cifrado 4`: ningún valor sensible aparece en logs, errores ni exportación a terceros; sin llave no arranca.
- Fuzz nativo del formato del valor (se suma a nv-10).

## Criterios
- Rotar la llave no pierde datos.

## Límites
- La llave en SSM protege contra robo de la base, no contra alguien con acceso a la instancia.
- Sin KMS propio: el costo es cero.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 7: sin conexión y cifrado, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-48/checklists/gates.md` + `Memoria.md`.
