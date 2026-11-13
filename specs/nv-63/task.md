# task.md — NV-63 Postales, recuerdos y momentos

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec y decidir quién dibuja la postal y la tarjeta; nv-42 a nv-45 fusionadas; rama contra `main`.

## Fase 1 — Implementación
- [ ] T02 Colecciones `postales`, `recuerdos` y `momentos` con sus índices.
- [ ] T03 Postales: crear con `imagen_id`, álbum, detalle por visibilidad, `PATCH` y `DELETE` con `If-Match`, galería `/lugares/{id}/postales`.
- [ ] T04 Recuerdos: generar desde un plan pasado (propio o de grupo), leer, listar y borrar.
- [ ] T05 Momentos: publicar, feed de amigos, detalle, editar y borrar; etiquetados solo amigos.
- [ ] T06 Panel del negocio: listar, agradecer y destacar con aviso.
- [ ] T07 Moderación de imagen y texto, avisos nuevos y `Purger`/`Exporter` registrados.

## Fase 2 — Cierre
- [ ] T08 Pruebas, documentos y `Memoria.md`.
- [ ] T09 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd` y `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
