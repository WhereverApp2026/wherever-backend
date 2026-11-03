# NV-05 — Usuarios, perfil, gustos y privacidad (A1)

## Contexto
Guarda a cada persona que usa Wherever: registro, perfil, gustos, preferencias, privacidad y derechos ARCO. Origen: roles B6, US-012 y hoja A1 del Excel. Responsable de la hoja: Alexander Dominguez Zarate. Pantallas: P-06, P-09 a P-13, P-32, P-62, P-63 y P-65. **Depende de**: NV-04.

## Requisitos
1. Registro con correo (`POST /usuarios`): mayoría de edad, política de contraseña, rol `consumidor`, 50 puntos, correo de verificación y tokens para entrar de inmediato.
2. Validación en vivo de correo disponible, con límite de peticiones.
3. Perfil propio: leer y editar (nombre, foto, ciudad) con control de concurrencia `version` + `If-Match`.
4. Cambiar correo (pide contraseña, queda no verificado, reenvía verificación).
5. Gustos del onboarding (mínimo 3 categorías), preferencias, privacidad y teléfono (solo como HMAC-SHA-256).
6. Cambio de modo `explorar`/`negocio` según roles.
7. Autocompletado de ciudades.
8. Derechos ARCO: exportar mis datos, borrar mi historial, eliminar mi cuenta (30 días de gracia) y cancelar la eliminación, con un trabajo nocturno que ejecuta el borrado.

## Diseño
- Módulo `internal/usuarios` (modelo y repositorio, compartidos con auth) e `internal/usuarios/account/{handler,service,dto}`; `internal/ciudades/{handler,service,repository,dto}`; `internal/privacidad` para ARCO. Reutiliza `auth.Service.CreateSession` de `nv-04` para emitir tokens al registrarse.
- **Colección `usuarios`** completa (Excel A1 §1) con un cambio: se agrega `version` (int, obligatorio, inicia en 1), exigido por la constitución para `PATCH`/`PUT` con `If-Match`. Índices nuevos: `ux_codigo_amigo`, `ix_nombre_busqueda`, `ix_ubicacion`, `ix_eliminacion`, `ux_telefono_hash` (parcial). Ya existen `ux_correo` y `ux_google_id`.
- **Colección `ciudades`** (Excel A1 §2): índices `ux_ciudad` e `ix_centro`; carga inicial por script de importación (CSV o JSON), sin datos fijos en el código. Autocompletado por prefijo sobre `nombre_normalizado`, con `limite` 8 por defecto y máximo 50.
- **Teléfono**: se normaliza a E.164 y se guarda HMAC-SHA-256 con `PHONE_HMAC_KEY` (≥ 32 bytes, solo por entorno). Nunca se registra el número.
- **Contraseña**: Argon2id como en `nv-04`; la política (8 caracteres con letra y número) vive en el registro único de validators de `nv-02`.
- **Código de amigo**: 8 caracteres de un alfabeto sin ambigüedades, generado con `crypto/rand`, reintento ante colisión del índice único.
- **ARCO por registro**: `privacidad.Purger` y `privacidad.Exporter` son interfaces; cada módulo futuro registra la suya. `nv-05` entrega la mecánica y los registros de `usuarios` y `sesiones`; el historial de planes, check-ins y reseñas se conecta cuando existan esos módulos.
- **Trabajo nocturno** (borrado a 30 días y procesamiento de exportaciones): `gocron` con candado `lock:borrado_nocturno` en Redis para que, con 2 o más instancias, solo una lo ejecute. Cada ejecución registra cuántas cuentas procesó, sin datos personales.
- **Exportación**: crea un documento en `exportaciones` (`usuario_id, estado, creado_en`, una por día); el trabajo arma el JSON, y se avisa por correo con un enlace de descarga de un solo uso (D-2), no con el archivo adjunto. El archivo se borra al descargarlo y, si no, por TTL.
- **Cuenta con eliminación pendiente**: el guardia `usuarios.RequireActive` revisa el estado en cada petición protegida; solo deja pasar `GET /usuarios/yo`, `cancelar-eliminacion`, `logout` y `logout-todos`.
- Mayoría de edad: se calcula con la fecha del servidor en UTC; menos de 18 → 422 con mensaje claro.

## Contratos
Dieciséis endpoints. Detalle por fila en `docs/endpoints.md`.

| Método | Ruta | Auth | Códigos |
|---|---|---|---|
| POST | `/api/v1/usuarios` | no (5/h por IP) | 201, 409, 422, 429 |
| GET | `/api/v1/usuarios/correo-disponible?correo=` | no (limitado) | 200, 422, 429 |
| GET | `/api/v1/usuarios/yo` | JWT | 200, 401 |
| PATCH | `/api/v1/usuarios/yo` | JWT, `If-Match` | 200, 400, 401, 409, 422 |
| PUT | `/api/v1/usuarios/yo/correo` | JWT | 200, 401, 409, 422 |
| PUT | `/api/v1/usuarios/yo/gustos` | JWT consumidor | 200, 401, 403, 422 |
| PATCH | `/api/v1/usuarios/yo/preferencias` | JWT | 200, 401, 422 |
| PATCH | `/api/v1/usuarios/yo/privacidad` | JWT | 200, 401, 422 |
| PUT | `/api/v1/usuarios/yo/telefono` | JWT | 200, 401, 409, 422 |
| POST | `/api/v1/usuarios/yo/modo` | JWT | 200, 401, 403, 422 |
| POST | `/api/v1/usuarios/yo/exportar` | JWT | 202, 401, 429 |
| POST | `/api/v1/usuarios/exportaciones/descargar` | token del enlace (un solo uso, 24 h) | 200, 400, 410, 422, 429 |
| DELETE | `/api/v1/usuarios/yo/historial` | JWT | 204, 401, 503 |
| DELETE | `/api/v1/usuarios/yo` | JWT | 202, 401, 422 |
| POST | `/api/v1/usuarios/yo/cancelar-eliminacion` | JWT | 200, 401, 409 |
| GET | `/api/v1/ciudades/autocompletar?q=` | no | 200, 422 |

Toda ruta con cuerpo responde 400 si el JSON es ilegible. `DELETE /usuarios/yo/historial` responde 503 si falla un módulo registrado; es seguro reintentarlo porque el borrado es idempotente.

El usuario nunca incluye `password_hash`, `telefono_hash` ni `google_id`.

Una cuenta en `eliminacion_pendiente` solo puede llamar `GET /usuarios/yo`, `POST /usuarios/yo/cancelar-eliminacion`, `POST /auth/logout` y `POST /auth/logout-todos`; el resto con JWT responde 403. Versión vieja en `If-Match` da 409 y sin la cabecera da 400.

## Tests
- `Integration/Usuarios service 14` (en `tests/integration`, con bases reales): registro ok, menor de edad, contraseña débil, correo repetido (409), términos no aceptados, 50 puntos al crear, rol `consumidor`, gustos con 2 categorías (422), gustos primera vez no suma puntos extra de bienvenida duplicados, notificaciones de reservas no se apagan, modo negocio sin rol (403), teléfono hasheado, `If-Match` viejo (409) y sin cabecera (400), cambio de correo deja no verificado, eliminación exige `ELIMINAR`, cancelar fuera de plazo, código de amigo con colisión.
- `Unit/Ciudades 4`: normalización sin acentos, prefijo, límite, `q` de 1 letra (422).
- `Unit/Privacidad 5`: registro de `Purger` y `Exporter`, ejecución en orden, error de un módulo no corta a los demás (se reporta), borrado respeta los 30 días, candado Redis impide doble ejecución.
- `Integration/Usuarios 14` con bases reales: registro→sesión→`/yo`, correo duplicado en carrera (índice único), `PATCH` concurrente con `version`, `PUT` de gustos, preferencias parciales, privacidad, teléfono duplicado (409), cambio de modo, eliminar→cerrar sesiones→cancelar, trabajo nocturno borra solo vencidas, dos instancias solo una ejecuta el trabajo.
- `Integration/Ciudades 3`: autocompletado, índice usado, ciudad inexistente → lista vacía.
- `Security/Usuarios 8`: ningún campo sensible en las respuestas, teléfono jamás en logs, `correo-disponible` y registro con 429, un usuario no edita a otro, la descarga funciona una sola vez, vence a las 24 h y el archivo se borra al descargarlo, cuenta en `eliminacion_pendiente` solo responde `GET /yo`, cancelar, `logout` y `logout-todos` (lo demás 403) y el JWT y el refresh de antes ya no sirven, mayoría de edad por UTC, inyección en `q` y `correo` inofensiva.
- `Contract/Usuarios`: respuestas coinciden con la tabla de Contratos.

## Criterios
- `go build ./...`, `go vet ./...` y todas las suites en verde.
- 16 filas nuevas en `docs/endpoints.md`; `check_endpoints.sh → OK`.
- `critic PASS`, cobertura ≥ 45 %, semgrep 0.
- Con dos instancias, el trabajo nocturno procesa cada cuenta una sola vez.

## Límites
- **Decisión D-1** heredada de `nv-04`: si se aprueba `perfil_completo`, `nv-05` agrega la regla 403 `perfil_incompleto` y que `PATCH /usuarios/yo` acepte `fecha_nacimiento` solo mientras esté vacía.
- **Decisión D-2 (exportación), aprobada el 2026-11-03.** El JSON no se manda adjunto por correo: se manda un enlace de descarga de un solo uso con vencimiento de 24 h (tipo `descargar_exportacion` en `tokens_un_uso`, ruta `POST /usuarios/exportaciones/descargar`). Mientras no haya proveedor de correo, ese enlace no llega a nadie.
- **Excepción de stack aprobada**: `gocron` v2.22.0 (MIT) para el trabajo nocturno, registrada en `Memoria.md`.
- El borrado en cascada real (reservas, reseñas anonimizadas, grupos) y el historial de planes, check-ins y señales se conectan módulo por módulo con `Purger` y `Exporter`; hoy solo existen `usuarios` y `sesiones`. Cada módulo futuro debe registrar los suyos en su spec.
- Límite conocido: `exportaciones.datos_json` guarda el archivo como texto en el documento, con tope de 16 MB de MongoDB; cuando existan más módulos habrá que moverlo a otro almacenamiento.
- Sin subida de foto: `foto_url` se recibe como texto y se valida como URL del CDN; el almacenamiento (S3) es otro módulo.
- Sin cuentas de comerciante ni negocios (`negocios_ids` queda vacío); sin amigos (el código de amigo solo se genera).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (T1–T8 ejecutadas; sin firma)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-03 (ejecución)
- **Decisiones del revisor (2026-11-03):** D-2 aprobada (enlace de un solo uso de 24 h, tipo `descargar_exportacion`); `gocron` aprobado como excepción de stack (`github.com/go-co-op/gocron/v2` v2.22.0, MIT); D-1 sigue pendiente (sin `perfil_completo`).
- **Detalle:** evidencia en `specs/nv-05/checklists/gates.md` + `Memoria.md`. Desvíos:
  - Paquetes: `internal/usuarios` (modelo y repositorio, que también usa auth) y `internal/usuarios/account` (service, handler, DTO); juntos harían un ciclo de imports con `internal/auth`. `internal/ciudades` y `internal/privacidad` como dice la spec.
  - Versión vieja en `If-Match` = **409** y sin `If-Match` = **400**: la constitución §5 no admite 412 ni 428 (choque de versión es 409).
  - Una cuenta en `eliminacion_pendiente` **sí** puede iniciar sesión: al pedir la eliminación se cierran todas las sesiones, y volver a entrar es la única forma de cancelar. Con esa sesión solo responden `GET /usuarios/yo`, `cancelar-eliminacion`, `logout` y `logout-todos`; lo demás con JWT da 403 (`usuarios.RequireActive`, aprobado por el revisor). La prueba de seguridad verifica eso y que el JWT y el refresh de antes ya no sirvan.
  - Los 50 puntos de bienvenida se dan al registrarse; `PUT /gustos` no suma otros (la hoja A1 v3 ya se corrigió así).
  - Se agrega la ruta 16 `POST /api/v1/usuarios/exportaciones/descargar` (D-2) y la colección `exportaciones` con `ix_exportacion_estado`, `ix_usuario_exportaciones` y `ttl_exportacion` (el archivo se borra al descargarlo; límite de 16 MB por documento anotado en `Memoria.md`); `privacidad.buscable_por_telefono` se guarda junto a los demás interruptores. Faltan en el Excel.
  - Catálogos provisionales (sin definir en la hoja A1): movilidad `a_pie|bicicleta|auto|transporte_publico`, tema `claro|oscuro|sistema`, unidades `km|mi`; nivel inicial `Explorador`.
  - El autocompletado ordena por `con_cobertura` y nombre; "las más cercanas a la persona" queda para cuando exista la ubicación del usuario.
  - Las pruebas del service viven en `tests/integration` (bases reales); `Unit/Privacidad` cubre el registro y el orden, y el candado y los 30 días se prueban con Redis y MongoDB reales.
