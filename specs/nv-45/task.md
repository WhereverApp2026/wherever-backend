# task.md — NV-45 Subida, validación y publicación de imágenes

## Fase 0 — Preparación
- [x] T01 Rama `nv-45-imagenes` desde `origin/nv-45` con `main` fusionado (la firma de la spec sigue pendiente).

## Fase 1 — Implementación
- [x] T02 `POST /imagenes/subida`: formulario firmado (JPEG, PNG o WebP, 1 byte a 5 MB, 5 minutos).
- [x] T03 El original cae en `cuarentena/`.
- [x] T04 Confirmar: valida, recodifica sin EXIF en 1080 y 320, modera y publica, rechaza o manda a la cola de nv-43; la decisión del administrador publica o borra.
- [x] T05 ARCO: borrar la cuenta borra sus imágenes; exportación con `imagenes`. Las del negocio, con nv-55/56.
- [x] T06 Límite por usuario: 20 por hora y 50 por día.

## Fase 2 — Cierre
- [x] T07 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `docs/aviso-privacidad.md` y `Memoria.md`.
- [x] T08 Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_sdd`, `check_endpoints`; suites con contenedor en el CI del PR; evidencia en `gates.md`.
