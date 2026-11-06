# gates.md — NV-12 Dos instancias de la API detrás de un balanceador y CloudFront

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | salida de `critic` | pendiente |
| critic B2 | Sin tokens en logs | salida de `critic` | pendiente |
| critic B3 | Cambios en `cd-dev.yml` aprobados por el revisor | salida de `critic` + aprobación | pendiente |
| critic B4 | Endpoints sin cambios | salida de `critic` | pendiente |
| Organización | ELB y CloudFront no bloqueados por política | Balanceador, grupo de destino y distribución creados y borrados el 2026-11-05; quedan 0 | listo |
| IP del cliente | Cabecera falsificada no evade el límite; pruebas con 1, 2 y 3 entradas | `TestClientIP_*`, `TestIPCliente_FalsificarLaCabeceraNoEvadeElLimiteDeLogin` | listo en local |
| Compresión | Con `Accept-Encoding` llega comprimida por CloudFront; `/auth` y `POST /usuarios` sin comprimir | `TestCompress_*`, `TestCompresion_*`; falta por CloudFront | listo en local |
| Auditoría | Sin estado local que rompa con dos instancias | `docs/auditoria-estado.md` | pendiente |
| Plan | Sin reemplazos inesperados | `terraform plan` | pendiente |
| Salud | Dos destinos sanos; HTTPS válido por CloudFront; HTTP redirige | salida de `curl` | pendiente |
| Aislamiento | El balanceador no responde directo desde internet | salida de `curl` | pendiente |
| Caída | Una API detenida, servicio continúa | salida | pendiente |
| Escalonado | B instala solo con A sana y con el SHA nuevo, visto por CloudFront; sin corte | corrida de `cd-dev` | pendiente |
| Regreso | Imagen rota regresa sola y B no la instala | corrida | pendiente |
| Petición larga | `/api/v1/probe/lenta` de 45 s responde por CloudFront | salida | pendiente |
| Sonda oculta | Al terminar la prueba, `/api/v1/probe/lenta` da 404 | salida de `curl` | pendiente |
| Puerto 22 | Cerrado en las dos APIs; instalación por Systems Manager | `verificar.sh` | pendiente |
| `cd-dev` apagado | Con todo apagado termina en verde con aviso; otro SHA sigue en rojo | corrida | pendiente |
| Estado | Sesiones y límites correctos con peticiones alternadas | salida | pendiente |
| Calendario | Encendido de las 08:00 deja ambas sanas | `/health` | pendiente |
| `verificar.sh` | exit 0 | salida | pendiente |
| Costo | Mensual y presupuesto documentados y aceptados | `README.md` | pendiente |
