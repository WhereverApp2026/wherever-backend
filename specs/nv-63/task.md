# task.md — NV-63 Postales, recuerdos y momentos

## Fase 0 — Preparación
- [x] T01 Aprobar la spec y decidir quién dibuja la postal y la tarjeta; nv-42 a nv-45 fusionadas; rama contra `main`. (Decisiones aplicadas como se proponen, sin firma: la app dibuja.)

## Fase 1 — Implementación
- [x] T02 Colecciones `postales`, `recuerdos` y `momentos` con sus índices.
- [x] T03 Postales: crear con `imagen_id`, álbum, detalle por visibilidad, `PATCH` y `DELETE` con `If-Match`, galería `/lugares/{id}/postales`.
- [x] T04 Recuerdos: generar desde un plan pasado (propio o de grupo), leer, listar y borrar.
- [x] T05 Momentos: publicar, feed de amigos, detalle, editar y borrar; etiquetados solo amigos.
- [x] T06 Panel del negocio: listar, agradecer y destacar con aviso.
- [x] T07 Moderación de imagen y texto, avisos nuevos y `Purger`/`Exporter` registrados.

## Fase 2 — Cierre
- [x] T08 Pruebas, documentos y `Memoria.md`.
- [x] T09 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd` y `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
