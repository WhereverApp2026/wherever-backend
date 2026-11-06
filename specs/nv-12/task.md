# task.md — NV-12 Dos instancias de la API detrás de un balanceador y CloudFront

## Fase 0 — Preparación
- [x] T01 Revisor aprueba decisiones (a)–(e); NV-11 terminada.
- [ ] T02 Ampliar boundary y política del deployer en la consola, en ese orden.
- [ ] T03 Recurso de prueba de ELB y de CloudFront creado y borrado; confirmar que la organización no los bloquea.
- [ ] T04 Rama `nv-12-alta-disponibilidad`; respaldo del estado de Terraform.
- [ ] T05 El operador confirma el saldo de créditos en Billing; subir el presupuesto a unos US$50 con avisos al 50, 80 y 100 %.

## Fase 1 — IP del cliente (Req. 2)
- [x] T06 Implementar `TRUSTED_PROXY_HOPS` (con `configs/.env.example`) y el cálculo de la IP con caída a la dirección de la conexión.
- [x] T07 Pruebas con 1, 2 y 3 entradas, cabeceras falsificadas y límite de inicio de sesión.
- [ ] T08 PR propio, fusionar y desplegar.

## Fase 2 — Compresión, sonda y auditoría (Req. 3, 7)
- [ ] T09 Middleware de compresión con sus pruebas: `Accept-Encoding` y `/auth` sin comprimir.
- [ ] T10 Sonda `/api/v1/probe/lenta`, oculta por `HIDE_INTERNALS`.
- [ ] T11 PR propio, fusionar y desplegar; confirmar la compresión en el servidor.
- [ ] T12 Auditoría de estado local y `docs/auditoria-estado.md`.

## Fase 3 — Infraestructura (Req. 4, 5, 6, 10)
- [ ] T13 `alb.tf`, `cdn.tf` y segunda API; `plan` mostrado.
- [ ] T14 `apply` con confirmación; segundo `plan` sin diferencias.
- [ ] T15 IP elástica de la segunda API en Atlas.

## Fase 4 — Despliegue escalonado (Req. 8, 9)
- [ ] T16 `actualizar.sh` con salida y reingreso del balanceador y salvaguardas.
- [ ] T17 `cd-dev.yml` con la etiqueta de revisión en la imagen.
- [ ] T18 Instalación de B condicionada a A: salud en el grupo de destino y SHA nuevo en `/health` por CloudFront.
- [ ] T19 Instalar temporizador y script en la segunda API por Systems Manager; cerrar el 22 en las dos.
- [ ] T20 `cd-dev.yml` con la URL de CloudFront, espera de unos 10 min, lecturas seguidas y 5xx sostenidos como apagado; variable `API_URL`.

## Fase 5 — Pruebas (Req. 11)
- [ ] T21 Salud, certificado, redirección y aislamiento del balanceador.
- [ ] T22 Caída de una instancia.
- [ ] T23 Despliegue escalonado y regreso automático.
- [ ] T24 Compresión por CloudFront.
- [ ] T25 `REQUEST_TIMEOUT` en 55 s; petición larga con `/api/v1/probe/lenta` por CloudFront, y volver a ocultarla; comprobar que da 404.
- [ ] T26 Estado compartido entre instancias.
- [ ] T27 Calendario de apagado y encendido; `cd-dev` en verde con todo apagado.

## Fase 6 — Cambio de URL y cierre (Req. 12, 13)
- [ ] T28 Cambiar la URL de la app; quitar Caddy; `TRUSTED_PROXY_HOPS=2`.
- [ ] T29 `verificar.sh`, `README.md`, costo y presupuesto, enmienda de la constitución, `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- [ ] T30 `go build`, `go vet`, cinco suites, `actionlint`, `check_sdd`, `critic`; PR; evidencia en `gates.md`.
