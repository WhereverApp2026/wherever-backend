# NV-45 — Subida, validación y publicación de imágenes

## Contexto
La persona sube una foto directo a S3; la API la valida, la modera y la publica sin metadatos. **Origen**: TEC-32. **Depende de**: NV-44.

## Requisitos
1. `POST /imagenes/subida` devuelve una URL firmada de subida con límites: JPEG, PNG o WebP, 5 MB como máximo.
2. La imagen cae en un prefijo de cuarentena.
3. Si pasa, la API la **vuelve a codificar** (quita metadatos como GPS y EXIF) en dos tamaños (1080 y 320) y la publica; si no, la elimina o la deja en cuarentena.
4. Al eliminar la cuenta o el negocio se borran sus imágenes.
5. Límite por usuario: 20 subidas por hora.

## Diseño
- **URL firmada** con política (tipo y tamaño) generada con el rol de la instancia; vence en 5 minutos.
- **Procesamiento** en la API con la biblioteca estándar de imágenes de Go (decodificar y recodificar quita EXIF); máximo 4000 px por lado y límite de píxeles para evitar bombas de descompresión.
- **Colección `imagenes`:** dueño, estado (`cuarentena`, `publicada`, `rechazada`), ruta, tamaños, resultado de moderación.
- **Direcciones:** `https://<distribucion>.cloudfront.net/i/<id>/<tamano>.jpg`; el identificador es aleatorio.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/imagenes/subida` | JWT | 201, 422, 429 |
| POST | `/api/v1/imagenes/{id}/confirmar` | JWT dueño | 200, 404, 409, 422 |
| DELETE | `/api/v1/imagenes/{id}` | JWT dueño | 204, 404 |

## Tests
- `Unit/Imagen 10`: JPEG, PNG, WebP; extensión mentirosa; archivo de 6 MB; imagen enorme (bomba); archivo no imagen; EXIF con GPS (se quita).
- `Integration/Subida 8` con servidor local compatible con S3: URL firmada válida, vencida, tipo equivocado, confirmar sin subir (409).
- `Integration/Flujo 6`: subida → moderación → publicada; moderación rechaza; moderación caída deja en cuarentena.
- `Security/Imágenes 5`: el dueño ajeno no confirma ni borra; el bucket no es público; el prefijo de cuarentena no sale por CloudFront.
- En vivo: subir una foto real, verla por CloudFront y comprobar que no tiene EXIF.

## Criterios
- Las fotos publicadas no conservan metadatos.
- Nada sin moderación aprobada se sirve.

## Límites
- Sin edición ni recorte en la API.
- Solo fotos; sin video.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-45/checklists/gates.md` + `Memoria.md`.
