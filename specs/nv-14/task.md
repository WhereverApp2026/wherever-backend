# task.md — NV-14 Infraestructura como paso configurado

## Fase 0 — Preparación
- [x] T01 Confirmar con el docente workflow o scripts: bastan scripts.
- [x] T02 Rama `nv-14-infra-scripts`.

## Fase 1 — Scripts (Req. 1–7)
- [x] T03 `crear.sh` con plan, confirmación, respaldo, primer arranque, pasos manuales y espera de `/ready`.
- [x] T04 `destruir.sh` con plan, confirmación y respaldo.
- [x] T05 `actualizar.sh` sin exigir `/ready` si no hay versión instalada.
- [x] T06 `instalar-actualizador.sh --site`, con espera de `cloud-init` y `Caddyfile`.

## Fase 2 — Pruebas sin riesgo
- [x] T07 `--solo-plan` de ambos; sin confirmación no cambian nada.
- [x] T08 Instalador en el servidor vivo: sin primer arranque ni reinicio.

## Fase 3 — Cierre parcial (Req. 8, 9)
- [x] T09 `README.md`, enmienda de la constitución, `AGENTS.md`, `Memoria.md`.
- [x] T10 `check_sdd`, `critic`; PR. Sin cambios de Go: las suites las corre el CI.

## Fase 4 — Ciclo real
- [x] T11 Adaptar los scripts a lo que entregue NV-12.
- [x] T12 `destruir.sh` y `crear.sh` en la ventana acordada, antes de congelar la app móvil.
- [x] T13 Pasos manuales de Atlas, SNS y URL comprobados; evidencia en `gates.md`.
