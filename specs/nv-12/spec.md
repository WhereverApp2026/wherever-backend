# NV-12 — Dos instancias de la API detrás de un balanceador y CloudFront (HTTPS sin dominio)

## Contexto
Con Redis y Neo4j como servicios gestionados externos (NV-11), la API puede replicarse. Esta spec corre **dos instancias idénticas** de la API en zonas distintas detrás de un Application Load Balancer, y pone **Amazon CloudFront** delante para dar HTTPS con el certificado que AWS ya incluye en `*.cloudfront.net`, porque no hay dominio propio (ACM no valida `sslip.io`). El CD por sondeo de NV-08 sigue funcionando en cada instancia. **Depende de**: NV-11 terminada (sin Redis compartido, una segunda API no comparte revocaciones de sesión ni límites) y NV-08. Primera versión corregida por la revisión del 5 de nov de 2026.

## Requisitos
1. **Prerrequisito de organización:** antes de escribir Terraform, y ya con el boundary y la política del deployer ampliados (sin ellos no hay permiso de crearlos), se crea y se borra un recurso de prueba de Elastic Load Balancing y otro de CloudFront para confirmar que ninguna política de la organización los bloquea (como pasó con OIDC). Si alguno falla, se detiene y se decide.
2. **Corrección de seguridad previa, en su propio PR y antes de crear nada:** la IP del cliente se toma de `X-Forwarded-For` según un número de saltos de confianza (`TRUSTED_PROXY_HOPS`: 1 detrás de Caddy, 2 detrás de CloudFront y el balanceador). Se usa la entrada que está a esa distancia del final; si hay menos entradas que saltos, se usa la dirección de la conexión. Hoy la API toma la primera entrada y un cliente podría falsificarla para evadir el límite de peticiones y el bloqueo de inicio de sesión.
3. **Auditoría de estado local** de la API: limitador, cachés, archivos, cola de correo y tareas programadas. El borrado nocturno ya usa candado en Redis. El limitador solo usa memoria si Redis cae, aceptado en la constitución. La cola de correo en memoria se pierde si una instancia muere: se acepta por escrito (hoy el correo no sale).
4. Dos instancias de API (`wherever-dev-api-a` y `-b`) en dos zonas, con la misma imagen y parámetros, IP elástica cada una y su propio temporizador del CD. Puerto 8080 solo desde el grupo de seguridad del balanceador. **Puerto 22 cerrado en las dos APIs:** la instalación y las revisiones se hacen por Systems Manager, porque la IP del operador cambia con frecuencia y un 22 abierto es superficie de ataque sin necesidad.
5. **Balanceador** `wherever-dev-alb` con oyente HTTP 80 hacia un grupo de destino con chequeo de salud en **`/health`** (no `/ready`: no debe consultar Redis ni Neo4j cada 15 s ni sacar a las dos instancias por una pausa de un servicio externo). Umbrales 2 sanos y 2 no sanos, intervalo 15 s, vaciado de conexiones de 30 s. Su grupo de seguridad solo admite la entrada al puerto 80 desde la lista de prefijos administrada de CloudFront y no tiene otra regla de entrada; su salida se limita al 8080 de las APIs.
6. **CloudFront** `wherever-dev-api`: certificado por defecto, redirige HTTP a HTTPS, origen el balanceador por HTTP, todos los métodos, política sin caché (`CachingDisabled`) y política de solicitud al origen que reenvía todas las cabeceras del cliente salvo `Host` (incluida `Authorization`) y las cadenas de consulta, tiempo de espera de lectura del origen de 60 s (la generación de planes con IA podrá pasar de los 30 s predeterminados), clase de precio 100. `REQUEST_TIMEOUT` baja a 55 s en Parameter Store: con 60 s empataría con CloudFront y el cliente podría recibir su 504 en vez del error uniforme de la API. La URL pública pasa a ser `https://<id>.cloudfront.net`.
7. **Compresión:** como CloudFront con política sin caché no comprime y se quita Caddy, la API comprime sus respuestas. El middleware no existe todavía: se implementa en un PR propio, con la prueba de que respeta `Accept-Encoding` y de que no comprime lo que lleva tokens: `/auth` y `POST /usuarios`, que también los devuelve. Los cuerpos de menos de 200 bytes no se comprimen. Debe estar en producción antes de quitar Caddy.
8. **Actualización sin cortar el servicio:** cada instancia se saca sola del grupo de destino, instala y vuelve a registrarse. Nunca se saca si es el único destino sano, y siempre se vuelve a registrar al terminar aunque falle. La instancia B instala solo cuando la A está sana y sirve el SHA nuevo, y no antes de 5 minutos de retraso; si A regresó a la versión anterior, B no instala esa versión. B no llega directo a A, porque el 8080 solo admite al balanceador: lee la salud de A en el grupo de destino y consulta `/health` por la URL de CloudFront, y solo A puede responder con el SHA nuevo. Para que el actualizador conozca ese SHA, `cd-dev` agrega a la imagen la etiqueta `org.opencontainers.image.revision`; una imagen sin ella no la instala B, salvo que B aún no tenga ninguna versión, así que el primer despliegue con la etiqueta lo hace A sin comparar nada. El rol de la instancia recibe permiso de registrar y quitar destinos sobre ese grupo de destino y de leer su salud; el boundary se amplía igual.
9. `cd-dev` comprueba por la URL de CloudFront, espera hasta unos 10 minutos y exige varias lecturas seguidas de `/health` con el SHA nuevo. Si las instancias están apagadas, sigue avisando en verde: como CloudFront y el balanceador siempre responden, ya no hay "sin conexión", así que 502, 503 o 504 sostenidos durante toda la espera cuentan como apagado. Una instancia que responde con otro SHA sigue en rojo.
10. Atlas permite las IP elásticas de las dos APIs.
11. El apagado y encendido programados cubren ambas APIs. El balanceador y CloudFront permanecen activos.
12. **Transición sin cortar a nadie:** la URL `sslip.io` con Caddy sigue funcionando hasta que la URL de CloudFront pase todas las pruebas y la app cambie. Solo entonces se quita Caddy de las APIs y se pone `TRUSTED_PROXY_HOPS=2`. Mientras siga en 1, quien entra por CloudFront comparte el límite con los demás clientes del mismo punto de CloudFront: un 429 en esas pruebas no es un fallo de la spec si desaparece con el valor 2.
13. Costo documentado y presupuesto ajustado (alrededor de US$41 al mes con el balanceador, CloudFront dentro del nivel gratuito a verificar; unos US$29 más hasta el 2 de dic; presupuesto de unos US$50 con avisos al 50, 80 y 100 %). Antes del primer `apply` el operador confirma el saldo de créditos en Billing. El balanceador y la segunda instancia se destruyen después de la evaluación, salvo decisión contraria.

## Diseño
- `infra/aws-dev/alb.tf` (balanceador, oyente, grupo de destino, grupo de seguridad con la lista de prefijos de CloudFront), `infra/aws-dev/cdn.tf` (distribución) y ampliación de `main.tf` o `api.tf` para la segunda instancia.
- El grupo de seguridad del balanceador solo lleva la regla de la lista de prefijos de CloudFront, que cuenta como unas 55 reglas de la cuota por grupo (verificar la cuota); no se le agrega nada más.
- Sin secreto compartido entre CloudFront y el balanceador (evita un secreto en el estado de Terraform). Aun si otra distribución llegara al balanceador, no puede falsificar la IP real: la entrada de cliente que importa la agrega CloudFront y las demás defensas (JWT, límites) siguen aplicando. Una cabecera secreta queda como endurecimiento posterior.
- La actualización de cada instancia vive en `actualizar.sh` y usa la CLI de AWS con el rol de la instancia.
- Parámetros no secretos nuevos: `TRUSTED_PROXY_HOPS` (también en `configs/.env.example`, que las pruebas exigen para toda clave), `REQUEST_TIMEOUT`, el identificador del grupo de destino.
- Sonda `GET /api/v1/probe/lenta`: tarda 45 s y responde 200. Vive con las demás sondas, oculta por `HIDE_INTERNALS`. Hoy no hay un endpoint de planes con IA que probar; la prueba la activa solo mientras dura y la vuelve a ocultar, y un gate comprueba que quedó oculta.
- La zona de A es `us-east-2c`, porque moverla la reemplazaría; B va en `us-east-2a`. Caddy pasa a un perfil de Compose: sigue en A y B nunca lo levanta.
- Permisos: la política del deployer se amplía con `elasticloadbalancing` limitado por prefijo `wherever-dev-*` donde AWS lo permita y con `cloudfront` (`Resource: "*"`, porque AWS no permite acotar la creación), más la lectura de la lista de prefijos administrada y `ssm:SendCommand` con `ssm:GetCommandInvocation`, que reemplazan al SSH para instalar y revisar. El rol de la instancia solo recibe registrar y quitar destinos de su grupo de destino y leer su salud. Primero el boundary y después la política, en la consola.
- `instalar-actualizador.sh` y `verificar.sh` dejan de usar SSH y pasan a Systems Manager.
- Las invitaciones por enlace (US-098) usarán el dominio de CloudFront para los App Links; se resuelve en esa historia.

## Contratos
Sin endpoints nuevos. `GET /health` sigue devolviendo el SHA de la instancia que responde. Terraform: instancia ×2, balanceador, oyente, grupo de destino y adjuntos, grupo de seguridad, distribución. Salidas: URL de CloudFront. Enmienda de la constitución §3, §4 y §8 (CloudFront y balanceador, HTTPS con certificado de CloudFront en desarrollo, y compresión en el pipeline de la API).

## Tests
- `go build ./...`, `go vet ./...`, las cinco suites y `terraform fmt`, `validate`, `trivy config infra/` sin hallazgos altos.
- **IP del cliente:** con `TRUSTED_PROXY_HOPS` 1 y 2, cabeceras con una, dos y tres entradas y entradas falsificadas a la izquierda: la IP resultante es la real; con menos entradas que saltos, cae a la dirección de la conexión; el límite de inicio de sesión no se evade.
- `terraform plan` sin reemplazos inesperados (no reemplaza la EC2 existente ni su IP elástica).
- **Salud:** las dos instancias sanas en el grupo de destino; `https://<id>.cloudfront.net/health` da 200 con el certificado válido y HTTP redirige a HTTPS.
- **Aislamiento:** el balanceador no responde desde internet directo; solo por CloudFront.
- **Caída de una instancia:** detener una API; las peticiones siguen respondiendo.
- **Despliegue sin corte:** un commit a `main` se publica en ambas, una a la vez, sin errores sostenidos, y la B no instala hasta que la A está sana con el SHA nuevo.
- **Regreso automático:** una imagen rota en la A regresa sola y la B no la instala.
- **Compresión:** con `Accept-Encoding: gzip` por CloudFront la respuesta llega comprimida; sin esa cabecera, en `/auth` y en `POST /usuarios`, sin comprimir.
- **`cd-dev` con todo apagado:** termina en verde con aviso.
- **Petición larga:** `/api/v1/probe/lenta`, de 45 s, responde 200 por CloudFront.
- **Estado compartido:** inicio de sesión y refresco con peticiones alternadas entre instancias; el límite se comporta igual con dos instancias.
- **Calendario:** el encendido de las 08:00 deja ambas sanas.

## Criterios
- HTTPS válido sin dominio y servicio disponible con una sola API.
- La IP del cliente no es falsificable para evadir límites.
- Sin estado local que rompa con dos instancias.
- `terraform plan` limpio tras aplicar; costo y presupuesto documentados.

## Límites
- Entre CloudFront y el balanceador el tráfico va por HTTP (incluye el token JWT dentro de la red de AWS). Aceptado en desarrollo; con un dominio propio se pasa a HTTPS completo.
- La URL `cloudfront.net` cambia si se destruye y recrea la distribución: la app debe compilarse con la URL final. El ciclo de destroy y apply se hace antes del congelamiento (NV-14, TEC-35). Crear y borrar una distribución tarda varios minutos.
- Sin dominio, el correo sigue sin resolverse (SPF y DKIM); queda para cuando haya dominio.
- Sin autoescalado ni varias regiones. Los servicios de datos gratuitos no tienen alta disponibilidad. Solo desarrollo.
- El balanceador cobra aunque las instancias estén apagadas.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Infraestructura aplicada; faltan el despliegue escalonado, el regreso, el cambio de URL y la firma
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-05 (inicio de la ejecución)
- **Decisiones del revisor (2026-11-05):** aprobadas (a) CloudFront delante del balanceador, sin dominio; (b) costo de US$41 al mes, destrucción tras la evaluación y presupuesto de US$50; (c) puerto 22 cerrado e instalación por Systems Manager (cambió frente a la propuesta de abrirlo a la IP del operador); (d) ampliar boundary y política del deployer, con los JSON preparados por el revisor; (e) enmienda de la constitución. Si la prueba de ELB o CloudFront falla por verificación de cuenta o por la organización, no se insiste y se decide con el revisor.
- **Cambios frente a la versión anterior:** dominio y certificado de ACM sustituidos por CloudFront; chequeo de salud en `/health`; corrección de la IP del cliente; salida y reingreso del balanceador al instalar con instalación escalonada condicionada a que A sirva el SHA nuevo; espera de `cd-dev` ampliada; prerrequisito de verificación de políticas de la organización. Revisión contra el código: permisos antes de la prueba de la organización; compresión por implementar; sonda para la petición larga; `cd-dev` en verde con 5xx sostenidos; B comprueba a A por CloudFront y por la salud del grupo de destino, con etiqueta de revisión en la imagen; salida del balanceador al 8080; `REQUEST_TIMEOUT` de 55 s.
- **Detalle:** la evidencia irá en `specs/nv-12/checklists/gates.md` + `Memoria.md`.
