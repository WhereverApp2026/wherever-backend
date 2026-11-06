# gates.md — NV-12 Dos instancias de la API detrás de un balanceador y CloudFront

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | salida de `critic` | pendiente |
| critic B2 | Sin tokens en logs | salida de `critic` | pendiente |
| critic B3 | Cambios en `cd-dev.yml` aprobados por el revisor | salida de `critic` + aprobación | pendiente |
| critic B4 | Endpoints sin cambios | salida de `critic` | pendiente |
| Organización | ELB y CloudFront no bloqueados por política | Balanceador, grupo de destino y distribución creados y borrados el 2026-11-05; quedan 0 | listo |
| IP del cliente | Cabecera falsificada no evade el límite; pruebas con 1, 2 y 3 entradas | `TestClientIP_*`, `TestIPCliente_FalsificarLaCabeceraNoEvadeElLimiteDeLogin` | listo en local |
| Compresión | Con `Accept-Encoding` llega comprimida por CloudFront; `/auth` y `POST /usuarios` sin comprimir | Perfil con `gzip`; sin cabecera, registro y login sin comprimir | listo |
| Auditoría | Sin estado local que rompa con dos instancias | `docs/auditoria-estado.md` | pendiente |
| Plan | Sin reemplazos inesperados | 16 agregados, 6 cambios (solo etiquetas, horario y presupuesto), 2 borrados (22 y 8080 abiertos); ningún reemplazo; segundo `plan` sin cambios | listo |
| Salud | Dos destinos sanos; HTTPS válido por CloudFront; HTTP redirige | 2 `healthy`; `/ready` 200 con certificado válido; HTTP 301 | listo |
| Aislamiento | El balanceador no responde directo desde internet | `curl` directo sin respuesta | listo |
| Caída | Una API detenida, servicio continúa | B detenida: 9 respuestas 502 en los primeros 24 s, hasta marcarla no sana; luego 200 | listo |
| Escalonado | B instala solo con A sana y con el SHA nuevo, visto por CloudFront; sin corte | `ac665ef`: A a las 13:06 y B a las 13:12; 126 lecturas cada 2-3 s por CloudFront con 0 errores mientras B instalaba | listo |
| Regreso | Imagen rota regresa sola y B no la instala | A regresó en unos 3 min y la registró como fallida; B no salió mientras A no estaba sana y luego `A aún no sirve`; CloudFront 330 lecturas, 0 errores; alarma `regresos` en ALARM | listo |
| Petición larga | `/api/v1/probe/lenta` de 45 s responde por CloudFront | 200 en 45.4 s | listo |
| Sonda oculta | Al terminar la prueba, `/api/v1/probe/lenta` da 404 | 8 de 8 en 404 | listo |
| Puerto 22 | Cerrado en las dos APIs; instalación por Systems Manager | `verificar.sh` | listo |
| `cd-dev` apagado | Con todo apagado termina en verde con aviso; otro SHA sigue en rojo | corrida | pendiente |
| Estado | Sesiones y límites correctos con peticiones alternadas | Flujo completo 3 veces correcto. Con `TRUSTED_PROXY_HOPS=2` y una IP falsa distinta en cada intento de login, 429 y contador `rate:login:` de la IP real | listo |
| Calendario | Encendido de las 08:00 deja ambas sanas | `/health` | pendiente |
| `verificar.sh` | exit 0 | `verificar: OK` en las dos | listo |
| Costo | Mensual y presupuesto documentados y aceptados | `README.md`; US$41 al mes estimados, presupuesto de US$50; falta contrastar con el gasto real | listo |
