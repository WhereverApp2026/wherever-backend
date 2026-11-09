# NV-13 — Observabilidad en la nube (logs de eventos y errores)

## Contexto
La especificación del curso pide un servicio de observabilidad en la nube para manejar el log de eventos y errores. Hoy los logs de la API solo existen dentro de la instancia. Esta spec los envía a Amazon CloudWatch Logs, define métricas y alarmas de errores y avisa por correo. **Depende de**: NV-07 y NV-08; las alarmas del balanceador se agregan cuando exista NV-12.

## Requisitos
1. Los logs de la API van a CloudWatch Logs en el grupo `/wherever/dev/api`, con retención de 14 días. Cada instancia usa su propio flujo (`api-<id de la instancia>`, con la opción `awslogs-stream` de Docker). En modo `non-blocking`: si CloudWatch no responde, la API arranca y atiende igual, y pierde líneas de log, no peticiones.
2. La API ya registra en formato JSON estructurado; cada línea de acceso trae `level`, `time`, `ruta`, `metodo`, `estado`, `duracion_ms` y `request_id`. Nunca se registran cuerpos de petición, contraseñas, tokens ni correos completos (`critic B2`). El filtro actual solo mira el nombre de cada campo, y el texto de un error (por ejemplo, un `E11000` de MongoDB) puede traer un correo: el logger también oculta correos y credenciales de URI dentro de cualquier texto.
3. Filtros de métricas sobre el JSON: líneas con `level` = `ERROR` (en mayúsculas, como lo escribe `slog`), `estado` >= 500, tráfico, `duracion_ms`, dependencias caídas y regresos automáticos. Son 6 métricas; el plan gratuito cubre 10.
4. Alarmas con aviso a un tema de SNS suscrito al correo del proyecto: errores y 5xx por encima de un umbral en 5 minutos, `/ready` fallando y regreso automático. Sin datos no hay alarma, porque las instancias se apagan a diario. **Hoy nadie consulta `/ready` con frecuencia** (nv-12 chequea `/health`), así que esa alarma nunca sonaría: un timer `wherever-sonda` lo consulta cada 5 minutos en cada instancia, y la alarma cuenta la línea `dependencia no disponible`. Gasta unos 11 000 comandos de Redis al mes por instancia, el 2 % de la cuota. Las alarmas de hosts no sanos y 5xx del balanceador las agrega NV-12 cuando exista.
5. Consultas guardadas de CloudWatch Logs Insights para los casos comunes (errores recientes, peticiones lentas, intentos de inicio de sesión fallidos por ruta).
6. Un panel básico con errores, latencia y tráfico.
7. Presupuesto de volumen: el consumo de logs se mantiene bajo la cuota gratuita (5 GB de ingestión, almacenamiento y consultas de Insights combinados, según la página de precios de CloudWatch el 5 de nov. de 2026) con un máximo de 10 alarmas y 3 paneles. Se mide el volumen diario y se ajusta el nivel de registro si hace falta.
8. Los eventos de despliegue (digest, instalada, regreso automático) quedan en el mismo grupo, en un flujo `despliegues-<id de la instancia>`. Los escribe `actualizar.sh`, porque GitHub Actions no ve lo que pasa en el servidor. Un fallo al escribirlos nunca detiene una actualización.

## Diseño
- Controlador de logs `awslogs` de Docker en `server/compose.yml` (región, grupo, `awslogs-create-group` apagado: el grupo lo crea Terraform). Sin agente adicional. `deploy-env.sh` agrega `LOG_REGION` y `LOG_STREAM` a `.env`. Solo la API; Caddy sigue con sus logs locales.
- **Cambiar el controlador recrea el contenedor de la API y borra sus logs locales:** antes se revisa el trabajo nocturno pendiente de NV-15.
- `instalar-actualizador.sh` y `verificar.sh` pasan a Systems Manager (`ssm:SendCommand`, ya cargado para NV-12), porque el SSH depende de una IP que cambia y NV-12 cierra el 22.
- `infra/aws-dev/observabilidad.tf`: grupo de logs con retención, filtros de métricas, alarmas, tema de SNS y suscripción por correo, panel.
- Permisos: el rol de la instancia recibe `logs:CreateLogStream` y `logs:PutLogEvents` sobre ese grupo. El boundary `wherever-dev-boundary` se amplía con esos permisos, y las acciones de logs, métricas, alarmas y SNS van en una política administrada nueva, `wherever-dev-deployer-observabilidad`, adjunta al usuario: la del deployer ya pasa del límite de 6144 caracteres de AWS. Alarmas, paneles y SNS se limitan por prefijo `wherever-dev-*`; los logs, por `/wherever/dev/*`, porque el grupo no lleva ese prefijo. Las consultas guardadas, las descripciones y la lectura de métricas van con `Resource: "*"`, porque AWS no permite acotarlas.
- La suscripción de correo requiere que el operador confirme el mensaje de SNS.

## Contratos
Sin endpoints nuevos. Enmienda de la constitución §3: observabilidad en CloudWatch.

## Tests
- `go build ./...`, `go vet ./...` y las cinco suites sin cambios.
- `terraform fmt -check`, `validate`, `trivy config infra/` sin hallazgos altos; `terraform plan` solo agrega recursos y cambia el rol de la instancia.
- Una petición de prueba aparece en CloudWatch en menos de un minuto, con los campos esperados y sin datos sensibles.
- Unit: `TestLogger_*` (campos, llaves sensibles, correos y credenciales dentro de un texto).
- Un error provocado a propósito dispara la métrica y, pasado el umbral, la alarma y el correo. El servidor público oculta las sondas, así que el error se provoca con un contenedor aparte en otro puerto, con `HIDE_INTERNALS=false` y el mismo controlador de logs.
- Una consulta guardada de Insights devuelve resultados.
- Con la política ampliada, `verificar.sh` suma el grupo de logs y las alarmas.

## Criterios
- Logs de eventos y errores consultables en la nube, con alarma y aviso por correo.
- Sin datos sensibles en los logs.
- Consumo de logs bajo la cuota gratuita y costo adicional cero.

## Límites
- Solo la API; los logs de Atlas, Aura y Upstash se consultan en sus propios paneles.
- Sin trazas distribuidas ni APM.
- Al ser gratuito, retención corta y umbrales sencillos.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada en `main` (PR 24 y 25) y desplegada; sin firmar
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-05 (inicio de la ejecución)
- **Decisiones para el revisor antes de ejecutar:** (a) CloudWatch Logs con el controlador `awslogs`; (b) retención de 14 días; (c) correo del proyecto para las alarmas; (d) ampliar el boundary y la política del deployer; (e) enmienda de la constitución.
- **Detalle:** la evidencia irá en `specs/nv-13/checklists/gates.md` + `Memoria.md`.
