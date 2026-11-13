# NV-48 — Capa de cifrado de campos

## Contexto
Un paquete cifra y descifra campos sensibles con AES-256-GCM y llaves rotables. **Origen**: US-226. **Depende de**: NV-05.

## Requisitos
1. Cifrado por campo con AES-256-GCM para los datos sensibles que se guardan: alergias y fecha de nacimiento (el ánimo no se guarda; ver drift).
2. La llave vive en SSM como cadena segura (`/wherever/dev/DATA_KEYS`), con identificador de versión por llave para poder rotarla.
3. Cada valor guarda `enc:v1:<id_llave>:<nonce>:<texto cifrado>`; el nonce es aleatorio de 12 bytes y el dato asociado incluye colección, campo e id del documento para impedir copiar un valor a otro lugar.
4. Rotación: llaves nuevas cifran; las viejas siguen descifrando; un comando re-cifra todo (nv-49).

## Diseño
- **Paquete** `internal/platform/cifrado`: `CargarLlavero(DATA_KEYS)`, `Llavero.Cifrar(Lugar, texto)`, `Llavero.Descifrar(Lugar, valor)`, `EsCifrado` y `LlaveDe` (para rotar). `Lugar{Coleccion, Campo, ID}` es el dato asociado de GCM.
- **`DATA_KEYS`:** `id:base64,id:base64,…` con llaves de 32 bytes (`openssl rand -base64 32`); la primera cifra y todas descifran. Rotar = poner la nueva al principio, dejar la vieja y correr la migración de nv-49.
- **Fallo seguro:** sin `DATA_KEYS` la API no arranca en prod (`config.Load`); fuera de prod puede faltar y avisa (nv-49 guarda en claro). Una lista mal formada no arranca en ningún entorno.
- **Errores sin datos:** `ErrFormato`, `ErrNoAbre` y `ErrSinLlave` no llevan el valor ni dicen por qué no abrió (llave, dato asociado o alteración dan lo mismo).
- **Rendimiento medido:** cifrar y descifrar un valor toma unos 1,4 µs (`BenchmarkCifrarDescifrar`, i7-10870H).
- **Corrección de drift (2026-11-12):** el ánimo no se guarda en ningún lado: solo existe el consentimiento `privacidad.estado_animo` (un booleano). No hay valor que cifrar.

## Contratos
Sin endpoints nuevos. Variable nueva: `DATA_KEYS` (obligatoria en prod).

## Tests
- Unit `internal/platform/cifrado/cifrado_test.go`: ida y vuelta, nonce distinto cada vez, valor alterado, copiado a otro documento o a otro campo, llave equivocada, llave desconocida, rotación, llavero inválido (6 casos), formato inválido (5 casos) y benchmark.
- Fuzz `FuzzDescifrar`: ningún valor hace entrar en pánico al lector y solo el valor auténtico abre (se suma a nv-10).
- Unit `internal/platform/config/config_test.go` (`TestLoad_DataKeys`): sin llave no arranca en prod; en dev puede faltar; mal formada nunca arranca.
- Los datos sensibles en logs, errores y exportación se prueban con nv-49, que es quien los guarda.

## Criterios
- Rotar la llave no pierde datos.

## Límites
- La llave en SSM protege contra robo de la base, no contra alguien con acceso a la instancia.
- Sin KMS propio: el costo es cero.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12); implementada en la rama `nv-48`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** formato de `DATA_KEYS` (la primera cifra); en staging puede faltar; el ánimo no se cifra porque no se guarda.
- **Detalle:** la evidencia irá en `specs/nv-48/checklists/gates.md` + `Memoria.md`.
