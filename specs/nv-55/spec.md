# NV-55 — Alta de negocio por pasos y revisión del administrador

## Contexto
La persona da de alta su negocio en pasos con guardado automático (datos del negocio, ubicación, horarios, fotos, precios y servicios, documentos), captura sus datos fiscales y su CLABE, y lo manda a revisión. Un administrador ve la cola, revisa los documentos y aprueba o rechaza con motivo. **Origen**: hoja «A5 Negocios aliados» del Excel (responsable de la hoja: Alexander Dominguez Zarate; roles B12, pantallas P-40 a P-46 y «Cuenta para depósitos» de la maqueta 2). **Depende de**: NV-05 (usuarios, `RequireActive`, ARCO), NV-19 (`consentimientos`), NV-20 (`negocios`, estados, `qr_checkin`, `paso_alta`, `revision`), NV-40/41 (notificación al dueño), NV-44 (bucket privado) y NV-45 (subida de fotos), NV-48 (`internal/platform/cifrado`, `DATA_KEYS`). **La usan**: NV-56 (panel), NV-59 a NV-62.

## Requisitos
1. `POST /negocios` crea el negocio en `borrador` (paso 1: nombre, categoría, descripción) y agrega el rol `comerciante` a la cuenta si no lo tenía; no crea otra cuenta.
2. `PATCH /negocios/{id}/alta/{paso}` guarda los pasos 2 a 6 («Terminar después»): cada paso se valida solo y actualiza `paso_alta`.
3. Las fotos de la galería se suben con la subida de nv-45; deben quedar al menos 3.
4. La INE (frente y reverso) y el comprobante de domicilio se suben a la API, se cifran con la capa de nv-48 y se guardan en el bucket privado de nv-44, fuera de CloudFront.
5. `PUT /negocios/{id}/datos-fiscales` y `PUT /negocios/{id}/cuenta-deposito` validan RFC y CLABE y guardan cifrado.
6. Antes de subir documentos, datos fiscales o CLABE hay consentimiento expreso registrado en `consentimientos`; sin él, 422 `consentimiento_requerido`.
7. `POST /negocios/{id}/enviar-revision` pasa a `en_revision` si tiene lo mínimo; si no, 422 con la lista de lo que falta.
8. El administrador lista la cola (`en_revision`, del más antiguo al más nuevo), ve los documentos con registro de cada lectura, y aprueba o rechaza con motivo; el dueño recibe notificación.
9. Nada de la INE, el comprobante, los datos fiscales ni la CLABE sale en `/lugares`, en logs ni en caché; solo los leen el dueño y el administrador.
10. Retención: la INE se borra 90 días después de la revisión; los datos fiscales se conservan 5 años; ARCO completo.

## Diseño
- **Paquetes:** `internal/negocios/alta` (service, handler, DTO del alta y la revisión) y `internal/negocios/expediente` (repositorio de documentos y datos sensibles, cifrado, bitácora, `Privacy`). `internal/negocios` sigue con modelo, repositorio y catálogo; `alta` lo usa, no al revés. El service de usuarios se usa por interfaz para agregar el rol.
- **Pasos** (A5 §6): 1 datos (`nombre` 3–60, `categoria` de los 6 giros de nv-20, `descripcion` ≤ 300); 2 ubicación (`direccion`, `ubicacion` GeoJSON dentro de una ciudad de `ciudades`); 3 horarios (`horarios[]`, recalcula `horario_min` con el código de nv-20); 4 fotos (solo registra `fotos[]` ya publicadas por nv-45 con dueño = la persona); 5 precios y servicios (`nivel_precio`, `precio_promedio`, `servicios`, `opciones_dieta`, `alergenos`, `accesibilidad`); 6 verificación (estado de documentos, fiscales y CLABE; no recibe archivos). Cada `PATCH` exige `If-Match` con `version` (409 si no coincide, 400 si falta, como nv-05). Solo en `borrador` o `rechazado`; en otro estado, 409 `alta_cerrada` (el perfil aprobado lo edita nv-56).
- **Rol:** `POST /negocios` agrega `comerciante` a `usuarios.roles` (sin quitar `consumidor`). El access vigente conserva sus roles hasta el siguiente refresh; la respuesta lleva `requiere_refresh: true` y la app refresca antes del paso 2. Tope: 3 negocios en `borrador`/`en_revision` por cuenta (409 `limite_altas`).
- **Fotos:** `POST /negocios/{id}/fotos` pide la URL firmada de nv-45 (`POST /imagenes/subida` con destino `negocio:{id}`) y al confirmar la agrega a `fotos[]`; `DELETE /negocios/{id}/fotos/{foto_id}` (422 si quedan menos de 3 en un negocio aprobado o en revisión) y `PUT /negocios/{id}/fotos/orden` (orden y portada). La moderación de nv-42/43 aplica a las fotos como a cualquier imagen.
- **Documentos (`POST /negocios/{id}/documentos`):** multipart con `tipo` (`ine_frente`, `ine_reverso`, `comprobante_domicilio`) y `archivo` (JPEG, PNG o PDF; ≤ 5 MB; tipo real por los primeros bytes, no por la extensión). Pasa por la API, no por URL firmada: la API cifra los bytes con `cifrado` (dato asociado = `negocios_documentos`, `tipo`, id del negocio) y los sube a `privado/negocios/{id}/{tipo}/{aleatorio}.enc` en el bucket de nv-44, prefijo que la política de CloudFront no sirve (igual que la cuarentena de nv-45). Las imágenes se recodifican antes de cifrar para quitar EXIF y GPS; los PDF se guardan tal cual. No pasan por Rekognition ni por el modelo de lenguaje (sería transferir datos de identificación). Subir otra vez el mismo `tipo` reemplaza y borra el objeto anterior.
- **Colección `negocios_documentos`** (drift: A5 solo trae `documentos{rfc, comprobante_url}` en `negocios`; se separa para que nada sensible viaje con el documento de `negocios`, que leen `/lugares`, el caché de nv-18 y la exportación de favoritos): `negocio_id`, `propietario_id`, `tipo`, `llave_s3`, `content_type`, `tamano`, `subido_en`, `estado` (`vigente`, `borrado`), `borrar_despues_de` (INE: revisión + 90 días), `version`. Índices `ux_documento` (`negocio_id + tipo`, único) e `ix_borrar` (`borrar_despues_de`, parcial con `estado=vigente`). En `negocios` solo queda `documentos {ine: bool, comprobante: bool}` para el paso 6 y la cola.
- **Datos fiscales** (A5 §1 `datos_fiscales`): `rfc` (12 o 13 caracteres con la forma del SAT y homoclave; 422 `rfc_invalido`), `razon_social`, `regimen_fiscal` y `uso_cfdi` (claves del catálogo del SAT como constante), `cp_fiscal` (5 dígitos). Se guardan en `negocios_documentos` con `tipo=datos_fiscales`, todo el objeto cifrado en un campo (`datos_cifrados`), `conservar_hasta` = 5 años después de la baja del negocio.
- **CLABE** (A5 §7): 18 dígitos, banco por los 3 primeros (catálogo de Banxico como constante, autocompleta `banco`), dígito verificador con pesos 3-7-1; 422 `clabe_invalida`. Se guarda cifrada en `negocios_documentos` (`tipo=cuenta_deposito`) junto con `titular`; en `negocios.cuenta_deposito` solo `clabe_ultimos4`, `banco`, `titular` y `estado=sin_verificar`. A5 dice que la CLABE completa va a Stripe y no se guarda: nv-60 la registra como cuenta externa de Stripe Connect y, al confirmarse, borra la cifrada y deja solo los últimos 4. Hasta nv-60 la cifrada es la única copia.
- **Consentimiento:** `POST /usuarios/yo/consentimientos` de nv-19 con un tipo nuevo `datos_negocio` y la versión vigente del aviso. Documentos, fiscales y CLABE exigen ese registro (422 `consentimiento_requerido`). Retirarlo es la oposición o cancelación de ARCO (ver abajo). El aviso debe nombrar estos datos, su finalidad (verificar identidad y domicilio, facturar, depositar), los 90 días y los 5 años; la versión con estos datos sale de la rama `aviso-privacidad-integral` y sube `AVISO_VERSION` antes de desplegar.
- **Enviar a revisión:** exige paso 1 a 5 completos, ≥ 3 fotos publicadas, `ine_frente`, `ine_reverso` y `comprobante_domicilio` vigentes y consentimiento. Fiscales y CLABE no frenan la revisión (A5: «se pueden pedir después»); los pide nv-59 al activar anticipos. Escribe `revision.enviado_en` y `estado=en_revision` con un `findOneAndUpdate` condicionado a `borrador|rechazado` y la versión.
- **Cola del administrador:** `GET /admin/negocios?estado=en_revision` con cursor de nv-17 sobre `ix_revision` (A5); cada fila trae datos públicos, `documentos{ine, comprobante}`, `paso_alta` y días en espera; nada sensible.
- **Lectura del administrador:** `GET /admin/negocios/{id}/documentos/{tipo}` descifra y devuelve el archivo (o el JSON en `datos_fiscales` y `cuenta_deposito`) con `Cache-Control: no-store` y `Content-Disposition: attachment`; se agrega a `carriesTokens` para que no se comprima ni se guarde. Antes de responder escribe en **`accesos_expediente`**: `admin_id`, `negocio_id`, `tipo`, `motivo` (query obligatorio, 10–200), `ip`, `request_id`, `leido_en`; índices `ix_acceso_negocio` (`negocio_id + leido_en`) y TTL de 5 años. Si el registro falla, no se entrega el archivo (fail-closed, 503). El dueño lee los suyos por `GET /negocios/{id}/documentos/{tipo}` sin bitácora; `GET /negocios/{id}/documentos` lista solo metadatos.
- **Decisión del administrador:** `PATCH /admin/negocios/{id}/revision` con `decision` (`aprobar`, `rechazar`) y `motivo` (obligatorio al rechazar, 10–500). `findOneAndUpdate` condicionado a `en_revision` y `version` (409 si otro admin ya decidió). Aprobar: `estado=aprobado`, `verificado=true`, invalida la ficha en caché (nv-18) y notifica (nv-40/41, tipo `negocio_revisado`). Rechazar: `estado=rechazado`, `revision.motivo_rechazo`, notifica con el motivo; el dueño corrige y reenvía. En ambos casos fija `borrar_despues_de` de la INE a `revisado_en + 90 días`.
- **Retención (trabajo nocturno de nv-05, mismo candado):** borra objetos y documentos con `borrar_despues_de` vencido (INE y comprobante a los 90 días; el comprobante sigue la regla de la INE salvo decisión contraria) y datos fiscales con `conservar_hasta` vencido. El borrado de S3 es real (`DeleteObject` sin versiones; el bucket de nv-44 no versiona el prefijo `privado/`).
- **ARCO:** acceso = `GET /negocios/{id}/documentos` y la exportación (`Exporter`: metadatos de documentos, datos fiscales y CLABE enmascarada; nunca los archivos, que se descargan aparte). Rectificación = volver a subir o `PUT`. Cancelación (borrar la cuenta o retirar el consentimiento) = baja del negocio, borra INE, comprobante y CLABE al momento y **bloquea** los datos fiscales (solo lectura del administrador con motivo) hasta `conservar_hasta`. Oposición = igual que cancelación para estos datos. La bitácora de accesos no se borra con la cuenta (es prueba de quién leyó).
- **Logs:** se agregan `clabe`, `rfc`, `razon_social` y `archivo` a las llaves que oculta el logger; nunca se registra un cuerpo de estas rutas.
- **Límites de peticiones:** política nueva «Expediente del negocio» en `docs/rate-limit-matrix.md`: 10 por hora por usuario (`rate:expediente:{usuario}`, ventana fija) para `POST /documentos`, `PUT /datos-fiscales` y `PUT /cuenta-deposito`, así no se prueban CLABE ni RFC en masa; las rutas del administrador usan la política `Admin`. Un `INCR` con TTL por petición: se suma a `TestPresupuestoRedis_*`.
- **Variables:** ninguna nueva; usa `DATA_KEYS` y el bucket de nv-44. Sin `DATA_KEYS` en dev y staging, estas rutas responden 503 (no se guarda en claro, a diferencia de nv-49).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/negocios` | JWT consumidor o comerciante | 201, 400, 401, 403, 409, 422 |
| PATCH | `/api/v1/negocios/{id}/alta/{paso}` | JWT dueño | 200, 400, 401, 403, 404, 409, 422 |
| POST | `/api/v1/negocios/{id}/fotos` | JWT dueño | 201, 401, 403, 404, 409, 422, 503 |
| DELETE | `/api/v1/negocios/{id}/fotos/{foto_id}` | JWT dueño | 204, 401, 403, 404, 409, 422 |
| PUT | `/api/v1/negocios/{id}/fotos/orden` | JWT dueño | 200, 401, 403, 404, 409, 422 |
| POST | `/api/v1/negocios/{id}/documentos` | JWT dueño | 201, 401, 403, 404, 409, 413, 422, 429, 503 |
| GET | `/api/v1/negocios/{id}/documentos` | JWT dueño | 200, 401, 403, 404 |
| GET | `/api/v1/negocios/{id}/documentos/{tipo}` | JWT dueño | 200, 401, 403, 404, 503 |
| PUT | `/api/v1/negocios/{id}/datos-fiscales` | JWT dueño | 200, 401, 403, 404, 422, 429, 503 |
| PUT | `/api/v1/negocios/{id}/cuenta-deposito` | JWT dueño | 200, 401, 403, 404, 422, 429, 503 |
| POST | `/api/v1/negocios/{id}/enviar-revision` | JWT dueño | 200, 401, 403, 404, 409, 422 |
| GET | `/api/v1/negocios/mios` | JWT comerciante | 200, 401, 403 |
| GET | `/api/v1/admin/negocios?estado=en_revision&cursor=` | JWT administrador | 200, 401, 403, 422 |
| GET | `/api/v1/admin/negocios/{id}/documentos/{tipo}?motivo=` | JWT administrador | 200, 401, 403, 404, 422, 503 |
| PATCH | `/api/v1/admin/negocios/{id}/revision` | JWT administrador | 200, 400, 401, 403, 404, 409, 422 |

403 = otro rol, negocio ajeno o cuenta en eliminación pendiente; un negocio ajeno da 404 para no confirmar que existe. 422 = paso fuera de 2–6, RFC o CLABE inválidos, archivo de otro tipo, falta de consentimiento o lo que falta para enviar. Las rutas `documentos`, `GET /admin/negocios` y `GET /admin/negocios/{id}/documentos/{tipo}` no están en A5 (drift documentado). Filas en la tabla viva de `docs/endpoints.md`.

## Tests
- `tests/integration/negocios_alta_test.go` con MongoDB, Redis y S3 local: alta completa paso a paso, `If-Match` viejo (409), rol agregado, tope de 3 altas, enviar con faltantes (422 con la lista), enviar completo, aprobar y rechazar con motivo, dos admins deciden a la vez (uno gana, otro 409), notificación al dueño.
- `tests/integration/negocios_expediente_test.go`: documento cifrado en S3 (los bytes guardados no contienen el original), reemplazo borra el anterior, EXIF quitado, PDF disfrazado de JPEG (422), sin consentimiento (422), sin `DATA_KEYS` (503), lectura del admin deja fila en `accesos_expediente`, bitácora caída (503 sin archivo), retención de 90 días y de 5 años con reloj inyectado, ARCO (borrar cuenta, exportar con CLABE enmascarada, fiscales bloqueados).
- `tests/security/negocios_alta_security_test.go`: cada ruta con 401 sin token, 403 con otro rol, dueño ajeno (404), consumidor no lee la cola ni documentos, `/lugares/{id}` y `/lugares` nunca traen `documentos`, `datos_fiscales` ni CLABE, el prefijo `privado/` no sale por CloudFront.
- `tests/database/indexes_test.go`: `ux_documento`, `ix_borrar`, `ix_acceso_negocio` (TTL), `ix_revision`. `tests/contract`: alta en borrador, cola del admin y la parte `negocios_expediente` de la exportación ARCO.
- Unit `internal/negocios/alta` y `expediente`: DTOs por paso, RFC (persona física y moral), CLABE (válida, dígito malo, banco desconocido), tipo real del archivo, faltantes para revisión, enmascarado.

## Criterios
- Ningún dato de la INE, el comprobante, los fiscales o la CLABE aparece en claro en MongoDB, S3, logs, caché ni en `/lugares`; cada lectura del administrador queda en la bitácora.
- Un negocio solo llega a `aprobado` por la decisión de un administrador.

## Límites
- Sin autocompletado de dirección (`GET /geo/direcciones`, A5 §7): espera al proveedor de mapas (pendiente en `Memoria.md`).
- Sin «Que la IA me ayude» (`POST /negocios/{id}/ia/descripcion`): va con nv-56 o su propia spec.
- Sin registro en Stripe, depósito de prueba ni `cuenta-deposito/verificar`: son de nv-60, que extiende `PUT /cuenta-deposito`.
- Sin verificación automática de la INE contra el padrón del INE ni OCR: la revisa una persona.
- Sin edición del perfil aprobado ni pausa: nv-56.

## Implementación (2026-11-13) — drift resuelto
- **Paquetes:** `internal/negocios/alta` (service, handler, DTO, `Privacy` de la baja), `internal/negocios/expediente` (modelo, repositorio, cifrado, RFC, CLABE, archivo, `Privacy`, `Retencion`) y `internal/negocios/alta_repository.go` (consultas del alta sobre `negocios`). Armado en `cmd/api/alta.go`; pruebas con `authtest` (`AltaCompleta`, `AltaEnRevision`) y los dobles de `internal/testutil/negociostest`.
- **Fotos:** la subida, moderación y publicación son de nv-45 (`POST /imagenes/subida`). `POST /negocios/{id}/fotos` recibe `{imagen_id, tipo}` de una imagen ya publicada de la persona (interfaz `alta.Imagenes`); `PUT /fotos/orden` recibe `{fotos_ids[]}` y la primera es la portada (sin `portada_id`). Paso 4 no lleva cuerpo: exige 3 fotos. Las rutas de fotos aceptan cualquier estado y responden 409 si el negocio cambió a la vez. **Hasta que nv-45 llegue a `main`, `alta.Imagenes` no está conectada en `cmd/api` y las fotos responden 503.**
- **Almacén:** `expediente.Almacen` es el subconjunto `Leer/Guardar/Borrar` del almacén S3 de nv-45; **hasta que nv-45 llegue a `main`, `cmd/api` no lo conecta y la subida de documentos responde 503** (fiscales y CLABE no lo necesitan). En pruebas va el doble de S3.
- **Paso 2:** `ciudades` solo guarda el centro, no un polígono: la ubicación se valida dentro de México (mismas reglas de la semilla), no dentro de una ciudad.
- **Paso 6:** sin cuerpo; exige INE (frente y reverso) y comprobante vigentes.
- **Consentimiento:** `consentimientos` no tiene `tipo`. Se agregó `datos_negocio` (bool) a `POST /usuarios/yo/consentimientos`, a la fila de `consentimientos` y a `usuarios.privacidad`; vale si va con la versión vigente del aviso. Mandar `false` es la oposición: `expediente.Privacy.RetirarDatosNegocio` borra INE, comprobante y CLABE y bloquea los fiscales antes de guardar el retiro.
- **RFC:** forma del SAT (3 o 4 letras, fecha real AAMMDD, homoclave con dígito 0–9 o A); no se calcula el dígito verificador.
- **Archivos:** el tipo sale de los primeros bytes; un archivo de otro tipo con extensión `.jpg` da 422. Un PDF es válido aunque se llame `.jpg`.
- **Índices:** el TTL de 5 años de `accesos_expediente` va en `ttl_acceso` (`leido_en`), aparte de `ix_acceso_negocio`, porque MongoDB solo admite TTL en un índice de un campo. En `negocios_documentos` se suman `ix_conservar` (parcial `estado=bloqueado`) e `ix_propietario` (ARCO).
- **Baja por ARCO:** `alta.Privacy` da de baja (`eliminado_en`) solo los negocios con `origen: alta`; la semilla no lo lleva, así un administrador que borra su cuenta no se lleva el catálogo.
- **Aviso al dueño:** `negocio_revisado` lleva `{negocio_id, resultado}`; el motivo del rechazo se lee en `GET /negocios/mios` (los avisos solo llevan ids, nv-40).
- **Retención:** `privacidad.Job.WithTareas` corre `expediente.Retencion` con el mismo candado; si el borrado en S3 falla, el registro sigue y se reintenta.
- **Límite de peticiones:** `Expediente del negocio` usa `WindowUser` (el mismo `INCR` con TTL que `Buscar personas`, ya medido en `TestPresupuestoRedis_*`); son pocas peticiones por persona.
- **Pendientes:** conectar `alta.Imagenes` y `expediente.Almacen` con nv-45 en `cmd/api` (y la cola `en_revision` de las fotos de nv-42/43); el aviso con estos datos y `AVISO_VERSION` nueva (rama `aviso-privacidad-integral`) antes de desplegar; avisar a nv-60 de D3.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), ola 9; implementada con D1–D11 como se proponen. Fotos y subida de documentos responden 503 hasta conectar nv-45 (ver «Implementación»). nv-44 ya está: CloudFront solo sirve `i/*`, así que `privado/` queda fuera.
- **Revisores:** Osmar Reséndiz (pendiente de firmar D1–D11)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** D1 documentos subidos a la API y cifrados con `DATA_KEYS` (no URL firmada); D2 colección aparte `negocios_documentos` en lugar de `negocios.documentos`; D3 `PUT /cuenta-deposito` en nv-55 con CLABE cifrada hasta que nv-60 la mande a Stripe (nv-60 la reclama hoy); D4 fiscales y CLABE no frenan la revisión; D5 el comprobante se borra a los 90 días como la INE; D6 el rol `comerciante` se agrega sin quitar `consumidor` y con `requiere_refresh`; D7 tope de 3 altas abiertas; D8 `motivo` obligatorio en cada lectura del admin y bitácora de 5 años; D9 sin `DATA_KEYS` = 503 (no guardar en claro); D10 política nueva «Expediente del negocio» de 10 por hora; D11 avisar a nv-60 del cambio de D3 si se aprueba.
- **Detalle:** evidencia en `specs/nv-55/checklists/gates.md` + `Memoria.md` al ejecutar.
