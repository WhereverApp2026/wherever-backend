# NV-63 — Postales, recuerdos y momentos

## 1. Objetivo
Convertir fotos de la salida en postales de 3 estilos, armar el recuerdo de un plan terminado y publicar postales como momentos para los amigos o el público. Toda imagen pasa por moderación antes de que otra persona la vea.

- **Origen:** roles C7 y hoja «J8 Postales y recuerdos» del Excel.
- **Depende de:**
  - NV-45 (subida, recodificación sin EXIF ni GPS y publicación de imágenes), sobre NV-44 (S3 y CloudFront).
  - NV-42 y NV-43 (moderación de imagen y texto, cola de revisión).
  - NV-30 (planes) y NV-37 (integrantes, para el recuerdo de un plan grupal).
  - NV-36 (amigos aceptados y bloqueos).
  - NV-40 y NV-41 (avisos de etiqueta, agradecimiento y destacado).
  - NV-21 (`/lugares/{id}`) y NV-17 (cursor firmado).

## 2. Alcance / No-alcance
- **Dentro:** colecciones `postales`, `recuerdos` y `momentos`; las rutas de J8 corregidas (§3); `Purger` y `Exporter` de los tres; índices y pruebas.
- **Fuera:**
  - Comentarios y reacciones de momentos (módulo social B2 del Excel, sin spec): `reacciones_total` queda en 0 hasta entonces.
  - Exclusión de amigos silenciados en el feed: J2 la menciona, pero nv-36 no tiene silenciar. Se agrega cuando exista.
  - `checkin_id`: no hay módulo de visitas; el campo se agrega con él.
  - `puntos_ganados`: no hay módulo de puntos; vale 0 hasta entonces.
  - Mapa de la ruta (`ruta_imagen_url`): pide un proveedor de mapas estáticos con costo propio.
  - Vaca: el gasto sale del plan.

## 3. Contratos API
Filas de J8 corregidas contra lo que existe. Al programar se copian a `docs/endpoints.md`.

| Método | Ruta | Entrada | Auth | Respuestas |
|---|---|---|---|---|
| POST | `/api/v1/postales` | `{imagen_id, estilo, texto?, plan_id?, negocio_id?, visibilidad}` | JWT consumidor | 202, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/postales` | `?plan_id=&negocio_id=&estilo=&mes=&cursor=&limite=` | JWT, dueño | 200, 401, 403, 422 |
| GET | `/api/v1/postales/{id}` | — | JWT, dueño o con permiso por visibilidad | 200, 401, 404 |
| PATCH | `/api/v1/postales/{id}` | `If-Match`; `{estilo?, texto?, visibilidad?}` | JWT, dueño | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/postales/{id}` | `If-Match` | JWT, dueño | 204, 400, 401, 403, 404, 409 |
| GET | `/api/v1/lugares/{id}/postales` | `?cursor=&limite=` | opcional | 200, 404, 422 |
| GET | `/api/v1/recuerdos` | `?mes=&cursor=&limite=` | JWT | 200, 401, 403, 422 |
| POST | `/api/v1/planes/{id}/recuerdo` | `{tarjeta_imagen_id?}` | JWT, dueño o integrante confirmado | 201, 200 (regenera), 401, 403, 404, 422 |
| GET | `/api/v1/planes/{id}/recuerdo` | — | JWT, dueño o integrante confirmado | 200, 401, 404 |
| DELETE | `/api/v1/recuerdos/{id}` | — | JWT, dueño | 204, 401, 403, 404 |
| POST | `/api/v1/momentos` | `{postal_id, texto?, visibilidad, etiquetados[]?, plan_id?}` | JWT consumidor | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/momentos/feed` | `?cursor=&limite=` | JWT | 200, 401, 403, 422 |
| GET | `/api/v1/momentos/{id}` | — | opcional; con permiso por visibilidad | 200, 404 |
| PATCH | `/api/v1/momentos/{id}` | `If-Match`; `{texto?, etiquetados[]?, visibilidad?}` | JWT, autor | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/momentos/{id}` | `If-Match` | JWT, autor | 204, 400, 401, 403, 404, 409 |
| GET | `/api/v1/negocios/{id}/momentos` | `?destacados=&cursor=&limite=` | JWT comerciante, propietario | 200, 401, 403, 404, 422 |
| PATCH | `/api/v1/negocios/{id}/momentos/{mid}` | `{agradecido?, destacado?}` | JWT comerciante, propietario | 200, 400, 401, 403, 404, 422 |

**Drift corregido frente al Excel:**
- **Subida:** `POST /postales/subida` se quita. La foto se sube con `POST /imagenes/subida` y `POST /imagenes/{id}/confirmar` de nv-45, que ya firma la URL, limita tipo y tamaño, quita EXIF y GPS, modera y publica. La postal recibe `imagen_id` en vez de `foto_key`.
- **Permisos:** ajeno, sin permiso por visibilidad o bloqueado = **404**, no 403, igual que nv-36 y nv-37, para no revelar que existe. 403 queda para la cuenta en eliminación pendiente y para el rol.
- **Autocompletado de `@`:** `GET /personas/buscar?solo_amigos=true` no existe. Se usa `GET /amistades?estado=aceptada&q=` de nv-36, sin ruta nueva.
- **Edición:** `PATCH` y `DELETE` de postales y momentos llevan `version` e `If-Match` (AGENTS §5).
- **"Dueño del negocio":** es el rol `comerciante` y `negocios.propietario_id` igual a quien llama.
- **Galería pública:** `/lugares/{id}/postales` sigue la forma de `/lugares/{id}` (nv-21). Negocio no aprobado = 404.
- **Menciones:** `menciones` se quita. Las personas se nombran con `etiquetados`; el texto con `@` es solo texto y no avisa a nadie.
- **Montos:** `gasto_total` en centavos.

## 4. Modelo de datos
- **`postales`:**
  - `usuario_id`, `imagen_id` (de `imagenes`, nv-45), `plan_id?`, `negocio_id?`.
  - `estilo` (`clasica`, `polaroid`, `pasaporte`), `texto?` (≤ 80), `visibilidad` (`privada`, `amigos`, `publica`).
  - `estado` (`en_moderacion`, `lista`, `rechazada`), `version`, `eliminado_en`, `creado_en`, `actualizado_en`.
  - `estado` refleja el de la imagen (`cuarentena` → `en_moderacion`, `publicada` → `lista`) y el texto moderado.
  - Índices: `ix_album {usuario_id: 1, creado_en: -1}`, `ix_postales_plan {usuario_id: 1, plan_id: 1}`, `ix_postales_negocio {negocio_id: 1, visibilidad: 1, creado_en: -1}`.
- **`recuerdos`:**
  - `usuario_id`, `plan_id`, `titulo`, `fecha`, `paradas[{nombre, hora, foto_url?}]` (copia fija del plan).
  - `postales_ids[]`, `gasto_total` (centavos, del `costo_total` del plan), `puntos_ganados` (0 por ahora), `tarjeta_imagen_id?`, `creado_en`, `actualizado_en`.
  - Índices: `ux_recuerdo {usuario_id: 1, plan_id: 1}` único e `ix_recuerdos_fecha {usuario_id: 1, fecha: -1}`.
- **`momentos`:**
  - `autor_id`, `postal_id` (único entre momentos vivos), `negocio_id?` (copiado de la postal), `texto?` (≤ 280), `visibilidad` (`amigos`, `publica`).
  - `etiquetados[]` (≤ 10, solo amigos aceptados), `plan_id?`, `reacciones_total`, `destacado_por_negocio`, `agradecido_por_negocio`, `version`, `eliminado_en`, `creado_en`, `actualizado_en`.
  - Índices: `ix_feed {autor_id: 1, creado_en: -1}`, `ix_etiquetados {etiquetados: 1, creado_en: -1}`, `ix_momentos_negocio {negocio_id: 1, visibilidad: 1, creado_en: -1}`, `ux_momento_postal {postal_id: 1}` parcial sin `eliminado_en`.
- **Reglas:**
  - **Postal:** solo con una imagen propia `publicada` o en `cuarentena`; ajena = 404; ya usada por otra postal = 409. `plan_id` debe ser un plan propio o de un grupo donde está confirmado.
  - **Feed:** momentos de amigos aceptados, de los más nuevos a los más viejos, hasta 500 amigos con `$in` sobre `ix_feed`, sin cuentas no activas ni bloqueadas.
  - **Momento:** solo de una postal `lista` (si no, 422) con visibilidad compatible; una postal `privada` se cambia a la del momento.
  - **Borrar una postal** borra su momento. **Borrar un recuerdo** conserva las postales.
  - **Recuerdo:** solo de un plan cuya fecha ya pasó (pestaña `pasados` de nv-30; si no, 422). Regenerarlo reemplaza la copia (200).

### Postal y tarjeta: quién dibuja
El Excel pide que el servidor aplique marco y estilo y genere la tarjeta de 1080 × 1920 con un trabajador y una cola.

**Propuesta:** la app compone la postal y la tarjeta y las sube como imagen normal por nv-45. El servidor guarda `estilo` como dato y solo valida, modera y publica.
- Evita fuentes y plantillas en el servidor, CPU en las dos instancias y una cola que se pierde al reiniciar.
- Cambiar el estilo = la app sube otra imagen y hace `PATCH` con el `imagen_id` nuevo.

**Alternativa:** dibujar en el servidor con `golang.org/x/image` en la cola acotada de nv-06. Suma una dependencia y un trabajo que reintente las postales atascadas en `en_moderacion`.

Decisión del revisor.

## 5. Seguridad
- **Quién ve una postal:**
  - `privada`: solo el dueño.
  - `amigos`: el dueño y sus amigos aceptados.
  - `publica`: cualquiera.
  - En todos los casos, solo en estado `lista` para quien no es el dueño, y nunca entre personas bloqueadas (nv-36).
- **Quién ve un momento:** igual, con `amigos` o `publica`. Etiquetar a alguien no le da acceso si no es amigo del autor.
- **Galería del negocio y panel `/negocios/{id}/momentos`:** solo `publica` y `lista`. Muestran `nombre_usuario` y foto de perfil del autor, nunca correo, teléfono ni ubicación.
- **La imagen:** nv-45 la recodifica y quita EXIF y GPS. El servidor nunca devuelve la ruta de cuarentena.
- **Moderación (nv-42 y nv-43):**
  - La imagen y los textos (`postales.texto`, `momentos.texto`) se evalúan antes de que otro los vea.
  - Dudoso = `en_moderacion` y va a la cola; rechazado = `rechazada`, visible solo para el dueño.
  - Si la moderación falla, cierra (no publica).
- **Avisos (nv-40 y nv-41):** tipos nuevos `etiqueta_momento`, `momento_agradecido` y `momento_destacado`, con texto genérico y sin la imagen ni el texto del momento.
- **Logs:** solo ids, estilo, visibilidad y estados. Nunca textos, URLs firmadas ni nombres.
- **Límites:**
  - Las subidas ya tienen 20 por hora (nv-45).
  - 30 momentos por hora por usuario (`rate:momentos:{usuario}`) y la política `writes`.
  - La galería pública usa el límite global.
- **ARCO (`nv-05`):**
  - `Purger.PurgeAccount`: borra postales, recuerdos y momentos del usuario (sus imágenes las borra nv-45) y lo quita de `etiquetados` en momentos ajenos.
  - `PurgeHistory` no los toca: son contenido del usuario, no historial de uso.
  - `Exporter` "postales", "recuerdos" y "momentos": metadatos propios con la URL pública de cada imagen, y los ids de momentos donde lo etiquetaron. Nunca contenido de otras personas.

## 6. Tests / Gates
- **`tests/integration/postales_test.go`** (MongoDB real; S3 y Rekognition simulados por interfaz, como en nv-45):
  - Crear con imagen propia; con imagen ajena o usada.
  - Visibilidad privada, amigos y pública, y bloqueados.
  - Álbum con filtros; `PATCH` con y sin `If-Match`.
  - Borrar con momento; galería pública solo con `lista`.
- **`tests/integration/recuerdos_test.go`:** plan pasado propio y de grupo; plan futuro = 422; regenerar; ajeno = 404; borrar conserva postales.
- **`tests/integration/momentos_test.go`:**
  - Publicar postal no lista = 422; feed solo de amigos.
  - Etiquetar a quien no es amigo = 422.
  - Panel del negocio: propietario, comerciante ajeno = 404, consumidor = 403.
  - Agradecer y destacar con aviso; límite 30/h.
- **ARCO:** borrar la cuenta y exportar en los tres módulos.
- **`tests/security/postales_security_test.go`:** las 17 rutas con 401 sin token donde aplica, 403 en eliminación pendiente y por rol, y 404 por visibilidad.
- **Otras suites:**
  - `tests/database/indexes_test.go`: índices de las tres colecciones.
  - `tests/contract`: formas y exportación.
  - Unit: DTOs, matriz de visibilidad, mapeo de estados y fecha de plan pasado.

## 7. Plan de entrega
- Requiere nv-42 a nv-45 fusionadas: sin imágenes ni moderación no hay postales.
- `go build`, `go vet`, Unit y lint en local; las cinco suites en el CI del PR.
- `scripts/indexes` y las filas en `docs/endpoints.md` (17 rutas).
- `curl`: galería pública sin sesión; postal ajena = 404.
- Tag tras aprobar.

## Aprobación
- **Estado:** 🚧 Borrador (2026-11-12), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:**
  1. La app dibuja la postal y la tarjeta, o el servidor (§4).
  2. Quitar `POST /postales/subida` y usar nv-45.
  3. 404 en vez de 403 por visibilidad.
  4. Sin `menciones`, mapa de ruta, puntos ni silenciados por ahora.
  5. La galería y el panel muestran `nombre_usuario` y foto del autor.
  6. 30 momentos por hora.
- **Detalle:** la evidencia irá en `specs/nv-63/checklists/gates.md` + `Memoria.md`.
