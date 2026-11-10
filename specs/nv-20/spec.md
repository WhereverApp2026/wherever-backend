# NV-20 — Negocios aprobados, categorías y carga semilla

## Contexto
Sin negocios en la base no hay nada que recomendar ni filtrar: esta spec crea el documento `negocios` que comparten las hojas A2 y A5, carga el catálogo del piloto y publica el catálogo de categorías que pinta los chips. **Origen**: US-152 (carga semilla), hoja A5 §1 (alta y control) y hoja A2 §1, §4 y §5 (modelo, índices y categorías). **Depende de**: NV-04 (rol `administrador` y middleware) y NV-05 (repositorio de usuarios y validadores). Corre antes que NV-17 y NV-18: no usa cursor firmado, `ETag` ni caché.

## Requisitos
1. **Colección `negocios` (A2/A5)**: un documento por negocio, con `propietario_id`, `slug` único, `nombre_normalizado`, un solo `categoria` (giro principal), `etiquetas[]`, `nivel_precio` 1–4, `ubicacion` GeoJSON, `direccion`, `horarios`, `fotos`, `calificacion`, `qr_checkin` único y los estados `borrador`, `en_revision`, `aprobado`, `rechazado` y `pausado`. Solo los `aprobado` y no eliminados son visibles.
2. **Carga semilla** (US-152): `POST /api/v1/admin/negocios/importar` recibe un CSV o un arreglo JSON de 1 a 100 negocios, valida fila por fila, reporta cada error con su fila y deja en `aprobado` los renglones válidos. Es idempotente por `slug`: repetir el archivo actualiza, no duplica.
3. **Categorías** `GET /api/v1/categorias`: devuelve los 6 giros de A2 con su conteo de negocios aprobados, las etiquetas y servicios más usados, y los filtros disponibles. No es el catálogo de 15 gustos del onboarding: en A1 los gustos son texto libre y pertenecen a NV-05.

## Diseño
- **Modelo:** el documento es exactamente A2 §1 más los campos de control de A5 §1 (`paso_alta`, `documentos`, `revision`, `cobros`, `qr_checkin`, `pausado`, `datos_fiscales`, `cuenta_deposito`). Esta spec escribe el subconjunto de A2 necesario para listar y recomendar, más `paso_alta=6`, `revision` y `qr_checkin`; los campos fiscales y de cobro llegan con la spec de A5.
- **Corrección al borrador:** se usan los nombres y valores del Excel, no los del stub. `categoria` es singular (no `categorias[]`), `nivel_precio` (no `precio_nivel`), `nombre_normalizado` (no `nombre_norm`), `calificacion` es un objeto (no un flotante), y los estados son `borrador`, `en_revision`, `aprobado`, `rechazado`, `pausado` (no `pendiente` ni `suspendido`). `acepta_alergias`, `accesible` y `mascotas` se sustituyen por `alergenos[]` y el objeto `accesibilidad{silla_ruedas, apto_ninos, pet_friendly}`.
- **Giros cerrados:** `restaurante`, `cafeteria`, `bar`, `entretenimiento`, `servicios`, `comercio`. El catálogo vive en `internal/negocios/catalog.go`; **no se crea una colección `categorias`**. Los conteos se calculan sobre `negocios`.
- **Extensiones documentadas:** `version` (concurrencia para el futuro `If-Match` de A5), `horario_min[]` (ventanas `{dia, desde, hasta}` en minutos desde las 00:00, con `hasta` mayor a 1440 para cruzar medianoche; lo consume NV-21) y `alergenos[]` (extension al A2 para el filtro duro de NV-23: `gluten`, `lacteos`, `huevo`, `soya`, `cacahuate`, `nueces`, `pescado`, `mariscos`, `ajonjoli`). `precio_promedio` va en pesos enteros, como A2; no se convierten a centavos.
- **Índices (A2 §1 + A5 §1):** `ix_geo_filtros` (`2dsphere` en `ubicacion` + `estado`,`categoria`,`nivel_precio`), `ix_texto` (texto en `nombre` peso 10, `etiquetas` peso 5 y `descripcion` peso 1, `default_language: spanish`), `ix_autocompletar` (`estado`,`nombre_normalizado`), `ix_categoria_calif` (`estado`,`categoria`,`calificacion.promedio` desc), `ux_slug` único, `ix_propietario`, `ix_destacados` parcial sobre `destacado_hasta`, `ux_qr_checkin` único e `ix_revision` (`estado`,`revision.enviado_en`). Se registran en `negocios.Indexes()` y `internal/indexes.All()`; **el único creador sigue siendo `scripts/indexes`** (no se agrega `scripts/indices_negocios`).
- **Visibilidad pública:** un filtro compartido `estado = "aprobado"` y `eliminado_en` ausente. `borrador`, `en_revision`, `rechazado`, `pausado` y los eliminados nunca cuentan ni salen. El objeto `pausado{desde,hasta,motivo}` de A5 es una pausa temporal que se aplicará cuando exista su endpoint; la semilla no lo escribe.
- **Semilla, formato:** `multipart/form-data` con el campo `archivo` o `application/json` con un arreglo. El formato se decide por la extensión `.csv`/`.json` (o por el `Content-Type` en el cuerpo JSON). El CSV debe traer el encabezado exacto; un BOM UTF-8 al inicio se descarta y las filas vacías se ignoran. Los topes son 100 negocios y el límite de cuerpo de 10 MB de la app (413); más de 100 filas o un encabezado distinto dan 422. El CSV usa `|` para listas y una columna por día:
  ```
  slug,nombre,categoria,descripcion,nivel_precio,precio_promedio,latitud,longitud,calle,numero,colonia,municipio,estado_republica,cp,telefono,lun,mar,mie,jue,vie,sab,dom,etiquetas,servicios,accesibilidad,opciones_dieta,alergenos,fotos,propietario_correo,calificacion_promedio,total_resenas,acepta_reservas
  ```
  `lun`…`dom` aceptan `HH:MM-HH:MM` o vacío; `dom` es el día 0 de Go y `lun` el 1. La semilla admite un tramo por día; el JSON admite varios. Las fotos son 3 a 10 URLs `https://`; la primera se marca `fachada`, la segunda `interior`, la tercera `producto` y las demás `interior`. El JSON usa los nombres del modelo y acepta `horarios[]` y `fotos[{url,tipo,orden}]` completos.
- **Semilla, validación:** `slug` 3–80 (`[a-z0-9]+(-[a-z0-9]+)*`), `nombre` 2–60, `descripcion` ≤160, categoría cerrada, `nivel_precio` 1–4, `precio_promedio` 0–100000, coordenadas dentro de México (lat 14–33, lng −118–−86), `cp` de 5 dígitos, teléfono E.164 opcional, al menos un día abierto, fotos 3–10, etiquetas/servicios ≤20 de 1–40 caracteres, dietas y alérgenos en sus listas, y `calificacion` coherente (0–5; con `total_resenas > 0` el promedio debe ser mayor que 0). Los aspectos que no lleguen se rellenan con el promedio. Un `slug` repetido dentro del archivo es error en la segunda aparición; un `slug` ya existente en la base se actualiza.
- **Semilla, dueño e idempotencia:** `propietario_correo` es opcional; si llega, debe ser un usuario `activo` y se usa su `_id`. Si falta, el dueño provisional es el administrador que importa; se verifica que su `sub` sea un ObjectId y que la cuenta esté activa. La escritura es `$set` de los campos de catálogo y `$setOnInsert` de `_id`, `qr_checkin`, `creado_en`, `calificacion` inicial, `cupones_activos=0`, `sello_excelencia=false`, `verificado=false` y `plan=gratis`; `version` sube y `actualizado_en` cambia. Reimportar re-aprueba y actualiza `revision`, pero no borra ni duplica, y no reinicia la calificación, los cupones ni el sello que ya existan. `qr_checkin` se genera con `crypto/rand` al crear y no cambia.
- **Semilla, errores:** los errores de validación no detienen el archivo; los renglones válidos se escriben y la respuesta es 200 con `{creados, actualizados, errores:[{fila, campo, mensaje}]}`. Un fallo de MongoDB detiene la importación con 503 (reintentar es seguro por ser idempotente). Los logs solo llevan conteos; nunca el correo del dueño ni los slugs.
- **Categorías, respuesta:** una agregación `$facet` sobre `negocios` visibles devuelve:
  ```json
  {
    "categorias": [{"id": "restaurante", "nombre": "Restaurantes", "conteo": 12}],
    "etiquetas": [{"id": "tacos", "nombre": "tacos", "conteo": 5}],
    "servicios": [{"id": "terraza", "nombre": "terraza", "conteo": 3}],
    "filtros": {
      "nivel_precio": [1, 2, 3, 4],
      "opciones_dieta": ["vegetariano", "vegano", "sin_gluten"],
      "accesibilidad": ["silla_ruedas", "apto_ninos", "pet_friendly"]
    }
  }
  ```
  `categorias` siempre trae los 6 giros en el orden de A2, con `conteo` 0 si no hay negocios. `etiquetas` y `servicios` solo incluyen valores con conteo mayor que 0, ordenados por conteo descendente y luego por `id`, con tope de 50. `filtros` es estático y sale del mismo catálogo. `Cache-Control: no-store`; sin `ETag` ni 304 hasta NV-18.
- **Módulo y rutas:** un solo `internal/negocios` con modelo, repositorio, catálogo, servicio, parser y handler; no se crea `internal/categorias` (el catálogo es del modelo) ni `internal/busquedas` (es NV-22). El handler registra las dos rutas. La ruta admin usa `limiter.Window("admin", 200, time.Minute)` antes de `middleware.RequireRoles(middleware.RoleAdmin)`; la de categorías usa el límite global.

## Contratos
| Método | Ruta | Auth | Rate | Respuestas |
|---|---|---|---|---|
| GET | `/api/v1/categorias` | no | global | 200, 503 |
| POST | `/api/v1/admin/negocios/importar` | JWT administrador | admin 200/min | 200, 400, 401, 403, 413, 422, 503 |

- `GET /categorias` no lleva token, no pagina y no devuelve `ETag`; el 503 es MongoDB caído.
- `POST /admin/negocios/importar` acepta `archivo` (multipart CSV/JSON) o un arreglo JSON; el 400 es cuerpo/formato ilegible, el 403 un consumidor, el 413 el límite de cuerpo, el 422 el encabezado/las filas y el 503 MongoDB. La respuesta es la de `Diseño`.
- Las filas y los campos de respuesta son `snake_case`. Las rutas se copian a `docs/endpoints.md` y la política `Admin` de `docs/rate-limit-matrix.md` pasa de “módulo admin (posterior)” a `nv-20`.

## Tests
- `Unit/Semilla 11`: CSV válido; encabezado distinto; BOM; categoría fuera del catálogo; precio/coordenadas fuera de rango; fotos insuficientes; horario inválido; slug repetido; listas con `|`; JSON con varios tramos; tope de 100 filas.
- `Unit/Categorías 4`: catálogo de 6 giros en orden; nombres de pantalla; filtros estáticos; mapeo de la respuesta con conteos de ejemplo (la exclusión real de estados se prueba en Integration).
- `Integration/Semilla 8` con MongoDB real: alta de 20–30 filas; segunda importación solo actualiza; un error de fila no impide las válidas; dueño por correo y dueño provisional; `qr_checkin` estable; `version` sube; `nombre_normalizado`/`horario_min` derivados; la calificación/cupones/sello no se pisan al reimportar.
- `Integration/Categorías 4`: conteos por categoría, etiqueta y servicio; los 6 giros con cero; `borrador`/`en_revision`/`rechazado`/`pausado`/eliminado excluidos; `no-store` y sin `ETag`/304.
- `Security/Catálogo 4`: la semilla sin token da 401; un consumidor da 403; el admin importa; ningún negocio no aprobado aparece en categorías.
- `Database/Negocios 2`: los 9 índices exactos y únicos/parcial/texto como A2/A5; `scripts/indexes` es idempotente y `/ready` falla si falta uno.
- `Contract/Negocios 2`: `categorias_200.json` e `importar_200.json` con `CONTRACT_CAPTURE=1`; sin la variable solo validan.
- Archivos: Unit en `internal/negocios/{catalog,import}_test.go` e `import_fuzz_test.go`; las demás en `tests/integration/negocios_test.go`, `tests/security/negocios_security_test.go`, `tests/database/indexes_test.go` (`TestIndexes_NegociosTextAndPartial` y las tablas comunes) y `tests/contract/negocios_contract_test.go`. Varios casos comparten una función de prueba.
- `Fuzz/Semilla 1`: `FuzzParseCSV` no entra en pánico; una entrada o produce un error o un `ImportRow` válido.
- Antes de programar: `scripts/check_sdd.sh` y `git diff --check`; después: `go build ./...`, `go vet ./...`, las cinco suites con `-count=1`, `critic`, `check_endpoints`, `check_coverage --min 45` y Semgrep en 0.

## Criterios
- Hay 20 a 30 negocios aprobados cargados desde el archivo del piloto; repetir la importación no crea duplicados y reporta `actualizados`.
- Un negocio en cualquier estado distinto de `aprobado`, o eliminado, nunca sale en los conteos públicos.
- `GET /categorias` devuelve los 6 giros, las etiquetas/servicios con conteo y los filtros estáticos; no hay 15 gustos inventados.
- Los 9 índices existen, son idempotentes y `/ready` los verifica; `scripts/indexes` sigue siendo el único creador.
- La semilla no escribe correos en logs ni en la respuesta de error; el dueño provisional es el admin solo cuando falta `propietario_correo`.
- `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `internal/http/openapi.json` y `Memoria.md` quedan al día.

## Límites
- No implementa el CRUD de A5, el alta del comerciante ni la revisión/aprobación manual: esos endpoints llegan con su spec. Los estados existen para que ese módulo los use; la semilla solo produce `aprobado`.
- No hay catálogo cerrado de 15 gustos: `usuarios.gustos.categorias` es texto libre (3–10, máx. 40) y se queda en NV-05. Si producto quiere una lista cerrada, se cambia A1/NV-05, no esta spec.
- No hay colección `categorias`, `internal/categorias` ni `internal/busquedas`; el catálogo es una constante y los conteos salen de `negocios`.
- No hay `ETag`, 304 ni caché Redis: NV-17 y NV-18 van en la ola 4. `GET /categorias` lleva `no-store` para no servir conteos viejos.
- No se agregan variables de entorno, configuración, scripts de índices, dependencias ni cambios de pipeline. El CSV real con datos del piloto no se versiona; `docs/semilla-negocios.csv` es una plantilla con ejemplos marcados.
- No hay almacenamiento de fotos, reseñas reales, recálculo de calificación, cupones, pagos ni datos fiscales. La semilla solo usa URLs `https://` y la calificación inicial del archivo.
- No hay `ciudad_id` en `negocios`: la ubicación es GeoJSON y `direccion.municipio/estado`; el filtro geográfico de NV-21 usa coordenadas.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada y probada (2026-11-10), cinco suites verdes; falta la firma y cargar el CSV real del piloto. Ruta prioritaria, ola 1: catálogo y recomendaciones
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-10
- **Decisiones para el revisor antes de ejecutar:**
  1. Confirmar que `GET /categorias` devuelve los 6 giros de A2 con etiquetas/servicios y filtros, y que **no** se inventa el catálogo de 15 gustos del stub.
  2. Confirmar los nombres del Excel (`categoria`, `nivel_precio`, `nombre_normalizado`, estados `borrador/en_revision/aprobado/rechazado/pausado`) frente a los del stub.
  3. Aprobar las extensiones `version`, `horario_min[]` y `alergenos[]`, y el dueño provisional administrador cuando falta `propietario_correo`.
  4. Aprobar la semilla idempotente por `slug` con errores por fila y éxito parcial, y el tope de 100 filas.
  5. Aprobar que A5 (CRUD, revisión, documentos, cobros) queda fuera y que la semilla solo deja `aprobado`.
  6. Confirmar que `GET /categorias` no usa `ETag`/caché y lleva `no-store` hasta NV-18.
- **Detalle:** la evidencia irá en `specs/nv-20/checklists/gates.md` + `Memoria.md`.
