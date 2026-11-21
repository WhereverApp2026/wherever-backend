# NV-76 — Insignia del Sello de Excelencia

## Contexto
Cuando el administrador otorga el Sello de Excelencia (nv-56, `PUT /admin/negocios/{id}/sello`), el dueño quiere mostrarlo en su local y en sus redes. **Origen**: hoja «A5 Negocios aliados» del Excel (`GET /negocios/{id}/sello/insignia`, diferida en nv-56) y la maqueta (`34_Sello`, dorado para el Sello). **Depende de**: NV-56 (sello y panel del dueño), NV-04/NV-05 (sesión y `RequireActive`).

## Requisitos
1. El dueño descarga la insignia de su negocio solo si tiene el Sello vigente.
2. La insignia es una imagen vectorial (SVG) que se imprime o comparte sin perder calidad, con el nombre del negocio y el año en que se otorgó.
3. El nombre del negocio nunca puede inyectar marcado en el SVG.

## Diseño
- **Quién:** JWT `comerciante` + `RequireActive`, dueño del negocio (ajeno o inexistente = 404, como el resto del panel).
- **Sin Sello:** `sello_excelencia` falso o sin `sello.otorgado` = 404 `no_encontrado` («el negocio no tiene el Sello de Excelencia»); el dueño ve el avance en el panel (nv-56).
- **Formatos:** sin `formato` o `formato=json` = 200 JSON `{negocio_id, nombre, otorgado_en, anio, svg}` (la app lo muestra y comparte); `formato=svg` = 200 `image/svg+xml` con `Content-Disposition: attachment; filename="sello-<slug>.svg"`. Otro valor = 422.
- **SVG:** generado en el servidor, sin plantillas externas ni fuentes remotas: círculo dorado, «Sello de Excelencia», el nombre del negocio (escapado con `html.EscapeString` y recortado a 40 runas con «…»), «Wherever» y el año de `sello.otorgado_en`. 512×512, `viewBox` fijo.
- **Cabeceras:** `Cache-Control: private, max-age=3600` y `ETag` débil con la `version` del negocio; la CSP global `default-src 'none'; frame-ancestors 'none'` también cubre el SVG (abierto directo en el navegador no corre scripts); el dibujo no usa estilos en línea.
- **Paquete:** `internal/negocios/panel` (`insignia.go`: armado del SVG; servicio y handler junto al QR).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/negocios/{id}/sello/insignia?formato=` | JWT dueño (comerciante) | 200, 401, 403, 404, 422 |

Fila del Excel (A5): `GET /api/v1/negocios/{id}/sello/insignia` «Descarga la insignia del Sello de Excelencia para el local y redes» (Dueño). Se copia a `docs/endpoints.md`.

**Drift con el Excel:** se agrega `formato` (JSON por omisión para la app, SVG para descargar). PNG queda fuera: la app puede rasterizar el SVG.

## Tests
- Unit `internal/negocios/panel/insignia_test.go`: el SVG es XML válido, lleva nombre y año, escapa `<script>`, `&` y comillas, recorta nombres largos.
- `tests/integration/negocios_insignia_test.go`: con Sello = 200 JSON y SVG con su `Content-Type`, `Content-Disposition` y ETag; sin Sello, retirado o ajeno = 404; `formato` inválido = 422.
- `tests/security/negocios_insignia_security_test.go`: 401 sin token o inválido; 403 consumidor; dueño ajeno = 404; nombre con `<script>` y `onload=` no sale sin escapar; CSP en la respuesta SVG.
- `tests/contract/negocios_insignia_contract_test.go`: forma del 200 JSON.

## Criterios
- Solo el dueño de un negocio con Sello vigente obtiene la insignia.
- Ningún nombre de negocio rompe ni inyecta el SVG.

## Límites
- Sin PNG ni PDF, sin códigos de verificación pública ni insignias por año.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-21); implementada en la rama `feat/nv-76-insignia-sello`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor (tomadas como las propone la spec):** SVG generado en el servidor; JSON por omisión y SVG con `formato=svg`; sin Sello = 404.
- **Detalle:** evidencia en `specs/nv-76/checklists/gates.md` + `Memoria.md`.
