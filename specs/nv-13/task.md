# task.md — NV-13 Observabilidad en la nube

## Fase 0 — Preparación
- [x] T01 Revisor aprueba decisiones (a)–(e).
- [x] T02 Rama `nv-13-observabilidad`; respaldo del estado de Terraform antes del `apply`.
- [x] T03 Ampliar el boundary y luego la política del deployer en la consola.

## Fase 1 — La API (Req. 2)
- [x] T04 Revisar los campos de log; el logger oculta correos y credenciales en texto libre.
- [x] T05 `TestLogger_*`: sin credenciales ni correos.

## Fase 2 — Infraestructura (Req. 1, 3, 4, 6)
- [x] T06 `observabilidad.tf`: grupo con retención, filtros, alarmas, SNS, panel.
- [x] T07 `plan` mostrado; `apply` con confirmación; segundo `plan` sin diferencias.
- [ ] T08 Operador confirma la suscripción de correo.

## Fase 3 — Envío de logs (Req. 1)
- [ ] T09 Revisar el trabajo nocturno de NV-15; `compose.yml` con `awslogs` y sonda de `/ready`; permisos del rol de la instancia.
- [ ] T10 Comprobar el flujo en CloudWatch.

## Fase 4 — Pruebas (Req. 4, 5, 7, 8)
- [ ] T11 Error provocado con un contenedor aparte: métrica, alarma y correo. Base caída: alarma de `/ready`.
- [ ] T12 Consultas guardadas de Insights.
- [ ] T13 Medir el volumen diario frente a la cuota gratuita.
- [ ] T14 Eventos de despliegue en el flujo `despliegues-<instancia>`.

## Fase 5 — Cierre
- [ ] T15 `verificar.sh`, `docs/observabilidad.md`, `README.md`.
- [ ] T16 Enmienda de la constitución, `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- [ ] T17 `go build`, `go vet`, cinco suites, `check_sdd`, `critic`; PR; evidencia en `gates.md`.
