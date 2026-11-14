# NV-56 — Panel del negocio: perfil, horarios, pausa, menú y reservas

## Contexto
Lo que el comerciante ve y edita de su negocio ya aprobado: el resumen del día (P-47), el perfil y los horarios (P-52), la pausa, el menú, la configuración de reservas, el QR de mostrador y la baja. Deja fijos los puntos que otras specs esperan: `PUT /negocios/{id}/reservas-config` (nv-59), la tarjeta de cupones de P-49 y el gancho de pausa (nv-61), la tarjeta de reseñas de P-50 y la regla del Sello (nv-62). **Origen**: hoja «A5 Negocios aliados» del Excel (responsable de la hoja: Alexander Dominguez Zarate; roles B12, pantallas P-47, P-49, P-50 y P-52; US-078 y US-154). **Depende de**: NV-55 (alta, expediente, rol `comerciante`), NV-20 (`negocios`, `horario_min`, `qr_checkin`), NV-17 (cursor), NV-18 (ficha en caché), NV-21 (`/lugares/{id}`), NV-45 (fotos del menú). **La usan**: NV-59, NV-60, NV-61 y NV-62.

## Requisitos
1. `GET /negocios/{id}/panel` da el resumen del día: reservas de hoy, próxima reserva, llegadas, cupones canjeados, calificación de la semana con su cambio y avance al Sello.
2. `PATCH /negocios/{id}` edita el perfil y los horarios con `If-Match`; cambiar nombre, dirección o ubicación pasa por revisión del administrador sin despublicar el negocio.
3. `POST /negocios/{id}/pausa` pausa o reanuda; mientras está pausado no sale en `/lugares`, recomendaciones, favoritos ni cupones, y avisa a los módulos registrados.
4. El menú se administra por platillo (`POST`, `PATCH`, `DELETE`) y se lee en público agrupado por sección, con búsqueda dentro del negocio.
5. `PUT /negocios/{id}/reservas-config` guarda «Horas y cupo» con los campos que lee nv-59.
6. `GET /negocios/{id}/qr` da el contenido del QR de mostrador; la app lo dibuja e imprime (E3).
7. `DELETE /negocios/{id}` da de baja (borrado lógico), salvo con reservas futuras (409).
8. El administrador otorga o retira el Sello de Excelencia con motivo.
9. Ninguna respuesta del panel trae datos del expediente de nv-55; la CLABE sale enmascarada y los fiscales solo como completos o no.

## Diseño
- **Paquete `internal/negocios/panel`:** service, handler y DTO del panel, perfil, pausa, reservas y baja; `internal/negocios/menu` para el menú. Leen y escriben `negocios` con el repositorio de `internal/negocios`; la propiedad (`propietario_id` = la persona y rol `comerciante`) se valida en el service. Un negocio ajeno da 404.
- **Panel (P-47):** el service junta fuentes registradas por interfaz, sin importar los módulos: `PanelFuente` con `Reservas` (nv-59: `reservas_hoy`, `proxima_reserva{hora, personas}`), `Cupones` (nv-61: `cupones_canjeados_hoy`, `cupones_activos`; es la tarjeta que abre P-49) y `Resenas` (nv-62: `llegadas_hoy`, `calificacion_semana`, `cambio_vs_semana_anterior`, `resenas_por_responder`; es la tarjeta que abre P-50). Una fuente sin registrar o caída deja su campo en `null` (la app muestra «—») y el panel responde 200. Día y semana en la zona de la ciudad del negocio (constitución). Agrega `estado`, `pausado`, `cambios_pendientes`, `datos_fiscales_completos`, `cuenta_deposito{clabe_ultimos4, banco, estado}` y `avance_sello`. Sin caché en Redis (cuota de Upstash); cada fuente tiene 1 s de tiempo.
- **Regla del Sello (A5 US-154, la espera nv-62):** un negocio es elegible si lleva ≥ 60 días aprobado, tiene ≥ 20 reseñas verificadas en los últimos 90 días con promedio ≥ 4.5 y ningún contenido rechazado por moderación en ese periodo. `avance_sello` = `{elegible, criterios[{nombre, actual, meta, cumple}]}`; días desde `revision.revisado_en` y el resto de `Resenas` (nv-62), que también cuenta los rechazos de moderación de sus reseñas (drift: `moderacion_cola` no guarda el negocio). Sin fuente, `actual` es null y no es elegible. No se otorga solo: `PUT /admin/negocios/{id}/sello` con `otorgar` y `motivo` (10–500) escribe `sello_excelencia`, `sello{otorgado_por, otorgado_en, motivo}`, invalida la ficha y notifica al dueño (`negocio_revisado` con `sello_otorgado` o `sello_retirado`).
- **Perfil (P-52):** campos de los pasos 1, 3 y 5 de nv-55 (`descripcion`, `categoria`, `horarios`, `nivel_precio`, `precio_promedio`, `servicios`, `opciones_dieta`, `alergenos`, `accesibilidad`, `telefono`) se aplican al momento; `horarios` recalcula `horario_min`. `nombre`, `direccion` y `ubicacion` (drift: A5 dice «vuelve a revisión») se guardan en `cambios_pendientes{campos, enviado_en}` y el negocio sigue `aprobado` con los datos de antes; entra a la cola de nv-55 con `GET /admin/negocios?estado=cambios_pendientes` y `PATCH /admin/negocios/{id}/revision` los aplica o los descarta con motivo. `If-Match` con `version` (409 viejo, 400 sin cabecera). Invalida la ficha en caché (nv-18) y sube `actualizado_en` para la sincronización (nv-46).
- **Pausa:** `POST /negocios/{id}/pausa` con `pausado` (bool), `hasta` (opcional, ≤ 90 días) y `motivo` (≤ 120). Pausar: `estado=pausado` y `pausado{desde, hasta, motivo}`; solo desde `aprobado`. Reanudar: `estado=aprobado` y quita `pausado`. El trabajo nocturno de nv-05 reanuda los que vencen `hasta`. Las reservas ya hechas no se cancelan (nv-59 decide si las avisa); no entran nuevas. Gancho **`negocios.Observador`** con `Pausado(ctx, negocioID)`, `Reanudado(ctx, negocioID)` y `DadoDeBaja(ctx, negocioID)`, registrado al armar la app: nv-61 pasa sus cupones a `pausado` (y no los reactiva solo al reanudar, lo decide el dueño); nv-59 bloquea disponibilidad. Un observador que falla no revierte la pausa: el evento queda en `avisos_pendientes` y la tarea `negocios_pausas` del trabajo nocturno lo reintenta.
- **Reservas (`PUT /negocios/{id}/reservas-config`, US-078):** `acepta_reservas` (bool), `capacidad_por_horario` (1–500 personas), `duracion_turno_min` (15–240, múltiplo de 15; también es el intervalo entre horarios), `anticipo{tipo: fijo|porcentaje|por_persona, valor}` (fijo y por persona en centavos ≥ 0; porcentaje 0–100), `politica_cancelacion` (`flexible`, `moderada`, `estricta`), `confirmacion` (`automatica`, `manual`) y `permite_cambios` (bool), los dos últimos propuestos por nv-59. Se guarda en `negocios.reservas_config` y copia `acepta_reservas` al campo de A2 que filtra `/lugares`. Un anticipo con `valor > 0` exige `cobros.estado=activo` (nv-60); hasta entonces 422 `cobros_no_activos`. `If-Match` como el perfil. Solo `aprobado` o `pausado`.
- **Menú (A5 §2):** colección `menu_items` con `negocio_id`, `seccion` (≤ 30), `nombre` (≤ 60), `descripcion` (≤ 140), `precio` (pesos, 0–100 000), `etiquetas` (`vegetariano`, `picante`, `sin_gluten`), `foto_url` (de `imagen_id`, imagen publicada por nv-45 del mismo dueño; se guarda también `imagen_id` para borrarla en la baja), `disponible`, `orden`, `version`, `creado_en`, `actualizado_en`. Índices `ix_menu` y `ix_menu_texto` de A5. Tope de 200 platillos por negocio (409 `limite_menu`). `PATCH` y `DELETE` con `If-Match`. `GET /lugares/{id}/menu?q=&seccion=` es público, solo de negocios `aprobado`, agrupa por sección en orden, oculta `disponible=false` y con `q` (2–60) usa el índice de texto; sin paginar.
- **QR (`GET /negocios/{id}/qr`):** `{contenido: "wherever://checkin/{qr_checkin}"}`; `qr_checkin` no cambia (nv-20). La app dibuja el QR (E3: sin biblioteca nueva ni excepción de stack; drift con A5, que pedía PNG); `Cache-Control: private, max-age=86400`.
- **Baja (`DELETE /negocios/{id}`):** cuerpo `confirmacion` = nombre exacto del negocio (422 si no coincide). Pregunta a `ReservasFuturas` (nv-59; sin registrar = ninguna) y responde 409 `reservas_futuras` si hay. Escribe `eliminado_en`, llama a `Observador.DadoDeBaja` y a la cancelación del expediente de nv-55 (INE, comprobante y CLABE se borran; fiscales bloqueados 5 años) y pide a nv-45 borrar las imágenes. El nombre y el `qr_checkin` quedan para reseñas y reservas pasadas.
- **Límites de peticiones:** sin política nueva; las globales y `Writes` de `docs/rate-limit-matrix.md`; la ruta del Sello usa `Admin`.
- **ARCO:** el `Exporter` del comerciante agrega sus negocios (perfil, horarios, menú y `reservas_config`); borrar la cuenta da de baja sus negocios como en la baja de arriba. El menú no tiene datos personales.
- **Variables:** ninguna nueva.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/negocios/{id}/panel` | JWT dueño | 200, 401, 403, 404 |
| PATCH | `/api/v1/negocios/{id}` | JWT dueño | 200, 400, 401, 403, 404, 409, 422 |
| POST | `/api/v1/negocios/{id}/pausa` | JWT dueño | 200, 401, 403, 404, 409, 422 |
| PUT | `/api/v1/negocios/{id}/reservas-config` | JWT dueño | 200, 400, 401, 403, 404, 409, 422 |
| POST | `/api/v1/negocios/{id}/menu` | JWT dueño | 201, 401, 403, 404, 409, 422 |
| PATCH | `/api/v1/negocios/{id}/menu/{item_id}` | JWT dueño | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/negocios/{id}/menu/{item_id}` | JWT dueño | 204, 400, 401, 403, 404, 409 |
| GET | `/api/v1/lugares/{id}/menu?q=&seccion=` | no | 200, 404, 422 |
| GET | `/api/v1/negocios/{id}/qr` | JWT dueño | 200, 401, 403, 404 |
| DELETE | `/api/v1/negocios/{id}` | JWT dueño | 204, 401, 403, 404, 409, 422 |
| PUT | `/api/v1/admin/negocios/{id}/sello` | JWT administrador | 200, 401, 403, 404, 422 |

403 = otro rol o cuenta en eliminación pendiente. 409 = versión vieja, estado que no permite la acción, tope del menú o reservas futuras. Filas de la hoja A5 §6 copiadas a `docs/endpoints.md` (sección de diferidos hasta que haya código).

## Tests
- `tests/integration/negocios_panel_test.go` con MongoDB y Redis: panel con fuentes de prueba registradas y sin registrar (`null`), una fuente lenta (1 s, sigue 200), perfil con `If-Match` viejo (409), horarios recalculan `horario_min`, cambio de nombre queda pendiente y el negocio sigue visible, el admin lo aplica, pausar y reanudar (oculto en `/lugares` y favoritos, observador llamado, reanudación por `hasta`), `reservas-config` válida y anticipo sin cobros (422), baja con y sin reservas futuras, baja borra el expediente, Sello otorgado y retirado.
- `tests/integration/negocios_menu_test.go`: alta, edición, orden, `disponible=false` oculto, búsqueda con acentos, tope de 200, negocio no aprobado (404 público).
- `tests/security/negocios_panel_security_test.go`: cada ruta con 401 sin token, 403 con consumidor, dueño ajeno (404), el panel no trae documentos, RFC ni CLABE completa, el Sello solo con administrador.
- `tests/database/indexes_test.go`: `ix_menu` e `ix_menu_texto`. `tests/contract`: panel, menú público, `reservas-config` y la exportación con negocios.
- Unit `internal/negocios/panel` y `menu`: DTOs, `reservas_config` (rangos y combinaciones), regla del Sello, QR que decodifica al mismo `qr_checkin`, confirmación de baja.

## Criterios
- Pausar oculta el negocio en todo lo público en la siguiente petición y avisa a los observadores; reanudar lo regresa.
- `reservas_config` guarda exactamente los campos que lee nv-59.
- El panel responde aunque falte una fuente y nunca expone el expediente.

## Límites
- Estadísticas (`GET /estadisticas`, `estadisticas_diarias` y su trabajo por hora), consejo de la IA (`GET /consejo`) y reporte mensual en PDF (`GET /reportes/mensual`): spec propia cuando existan reservas, check-ins y cupones.
- Facturas CFDI (`GET /facturas`, colección `facturas`, trabajo de facturación): spec propia; requiere elegir un PAC y una excepción de stack.
- Cobros, liquidaciones, saldo y estado de cuenta (`cobros/conectar`, `liquidaciones`, `saldo`, `estado-cuenta`): nv-60.
- Importar de Instagram (`POST /fotos/instagram`), insignia del Sello (`GET /sello/insignia`) y «Que la IA me ayude» (`POST /ia/descripcion`): diferidos.
- Sin plan premium ni límites por plan.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), implementada; sin firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones aplicadas (propuestas, sin firma):** E1 `cambios_pendientes` sin despublicar (la cola es `GET /admin/negocios?estado=cambios_pendientes` y la decisión el mismo `PATCH /admin/negocios/{id}/revision`); E2 regla del Sello como se proponía y solo el admin la otorga; E3 **sin PNG**: la API da el contenido y la app dibuja el QR (sin excepción de stack); E4 la pausa no cancela reservas y reanudar no reactiva cupones; E5 pausa ≤ 90 días; E6 anticipo > 0 exige `cobros.estado=activo`; E7 tope de 200 platillos; E8 confirmación con el nombre exacto; E9 diferidos como en «Límites».
- **Detalle:** evidencia en `specs/nv-56/checklists/gates.md` + `Memoria.md`.
