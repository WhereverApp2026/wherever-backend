# Aviso de privacidad integral — Wherever (vivo)

<!--
Nota interna, no se muestra a usuarios. Texto canónico del aviso: la app lo muestra y la persona lo
acepta con POST /api/v1/usuarios/yo/consentimientos (nv-19). Un cambio de fondo obliga a subir
AVISO_VERSION en Parameter Store y correr instalar-actualizador.sh en las dos instancias; así
privacidad.consentimiento_vigente pasa a falso y la app vuelve a pedir la aceptación.
Versión = AVISO_VERSION. Antes de publicar a usuarios reales: revisión jurídica (LFPDPPP, DOF 20/03/2025)
y, si el texto cambió desde la última aceptación, subir AVISO_VERSION.
-->

**Última actualización:** 20 de noviembre de 2026 · **Versión:** 2026-11

## 1. Quién es responsable de tus datos
**Osmar Yahir Reséndiz Vázquez** («Wherever», «nosotros»), con domicilio para oír y recibir notificaciones en Av. Universidad Tecnológica No. 1000, Col. El 61, El Carmen, Tula de Allende, Hidalgo, C.P. 42830, es responsable del tratamiento de tus datos personales.

Para cualquier asunto de privacidad, escríbenos a **yahiresendiz34@gmail.com**. Ahí atendemos las solicitudes de la sección 8.

## 2. Qué datos tratamos
Solo tratamos los datos que la app necesita para funcionar. A quien usa la app para explorar no le pedimos CURP, RFC, domicilio ni datos bancarios. Quien registra un negocio entrega además los datos de la sección 2.2.

### 2.1 Personas que usan la app

| Categoría | Datos | Cuándo los obtenemos |
|---|---|---|
| Cuenta | Nombre, nombre de usuario, correo electrónico, fecha de nacimiento, contraseña (guardada solo como huella Argon2id, nunca en texto), foto de perfil, ciudad | Al registrarte y al editar tu perfil. Si entras con Google, Google nos da tu nombre y tu correo verificado; no recibimos tu contraseña de Google |
| Teléfono | Solo una huella criptográfica (HMAC-SHA-256) del número, nunca el número; sirve para que tus amigos te encuentren si lo permites | Si decides registrarlo |
| Preferencias | Categorías que te gustan, rango de presupuesto, restricciones de dieta, movilidad, tema y notificaciones | Al configurar la app |
| Ubicación | Coordenadas del momento en que pides recomendaciones, un mapa o un plan. No guardamos un historial de tus ubicaciones | Solo si das permiso de ubicación |
| Actividad en la app | Planes que creas, búsquedas recientes, lugares que marcas como «No me interesa», amistades y solicitudes | Al usar la app |
| Sesiones y seguridad | Modelo y sistema del dispositivo, versión de la app, dirección IP, fechas de uso de cada sesión | Al iniciar sesión |
| Notificaciones | Identificador de tu dispositivo para enviarte avisos (token de notificaciones) | Si permites las notificaciones |
| Fotos y reseñas | Fotos que subes (sin metadatos como la ubicación GPS), reseñas, check-ins y recuerdos | Al publicarlos |
| Registros técnicos | Fecha, ruta, código de respuesta y duración de cada petición, sin contraseñas, tokens ni correos | Automáticamente, para seguridad y estabilidad |
| Pagos | Monto, fecha, estado y concepto de cada pago, y los últimos 4 dígitos y la marca de la tarjeta. **Wherever no recibe ni guarda el número completo de tu tarjeta ni su código de seguridad**: los captura y procesa Stripe directamente | Al pagar dentro de la app |

### 2.2 Personas que registran un negocio (comerciantes)
Para publicar un negocio y recibir pagos, verificamos que exista y que tú seas quien lo representa. Para eso pedimos:

| Categoría | Datos |
|---|---|
| Identificación | Identificación oficial vigente (INE), con tu nombre, fotografía, firma, CURP y domicilio que aparecen en ella |
| Domicilio | Comprobante de domicilio del negocio, no mayor a 3 meses |
| Datos fiscales | Nombre o razón social, RFC, régimen fiscal y domicilio fiscal, para emitir y recibir facturas |
| Cuenta de depósito | Banco, titular y CLABE de la cuenta donde recibes tus pagos |
| Datos del negocio | Nombre, giro, dirección, teléfono, horarios, fotos y descripción, que se publican en la app |

La INE, el comprobante de domicilio, los datos fiscales y la cuenta de depósito **no se publican**. Los guardamos cifrados, solo los ve el equipo que revisa las altas y los usamos únicamente para verificar tu identidad, cumplir obligaciones fiscales y pagarte. Los datos fiscales y bancarios son **datos patrimoniales**: te pedimos tu **consentimiento expreso** antes de recibirlos, en el paso del alta donde los entregas. Si no lo das, no podemos aprobar el negocio para recibir pagos.

### 2.3 Datos personales sensibles
Las **alergias alimentarias** son datos de salud, por lo tanto sensibles. Solo las guardamos si activas «Datos sensibles» en Privacidad, y solo las usamos para **no recomendarte lugares que no sean seguros para ti**. No las compartimos con nadie, tampoco con el proveedor de inteligencia artificial, y puedes borrarlas en cualquier momento.

El **estado de ánimo** que menciones al pedir un plan solo se usa si activas «Estado de ánimo», para ese plan, y no se guarda en tu perfil. Como forma parte de lo que escribes, viaja al proveedor de inteligencia artificial dentro de tu frase (sección 5).

## 3. Para qué usamos tus datos

**Finalidades necesarias** (sin ellas no podemos darte el servicio):
1. Crear y administrar tu cuenta, verificar tu correo y recuperar tu contraseña.
2. Mostrarte lugares, su horario, el clima y lo que tienes cerca.
3. Recomendarte lugares y armar planes según tus gustos, tu presupuesto, tu compañía y tus restricciones.
4. Guardar tus planes, búsquedas recientes y lugares descartados.
5. Conectar con tus amigos: solicitudes, búsqueda por nombre de usuario o teléfono, sugerencias de amistad y saber qué amigos van a un lugar.
6. Cobrar los pagos que hagas en la app y emitir sus comprobantes.
7. En negocios: verificar la identidad de quien lo registra y que el negocio exista, publicarlo, pagarle lo que le corresponde y cumplir obligaciones fiscales.
8. Proteger tu cuenta: bloquear intentos de acceso repetidos, cerrar sesiones y prevenir fraude y abuso.
9. Cumplir obligaciones legales y atender tus solicitudes de derechos.

**Finalidades opcionales** (puedes negarte sin perder el servicio):
1. Personalizar las recomendaciones con tu historial de uso («Aprendizaje personalizado»).
2. Enviarte promociones por correo («Promociones por correo»).
3. Medir el uso de la app con datos agregados para mejorarla («Datos de uso»).

Para negarte a una finalidad opcional, apágala en **Perfil → Privacidad** o escríbenos al correo de la sección 1. Si no estás de acuerdo con alguna al registrarte, puedes apagarla desde el primer momento.

## 4. Con quién compartimos tus datos
**No vendemos tus datos ni los compartimos con anunciantes.** Para operar la app usamos proveedores que tratan datos por nuestra cuenta y según nuestras instrucciones (encargados). Algunos están fuera de México, principalmente en Estados Unidos:

| Proveedor | Para qué |
|---|---|
| Amazon Web Services (Estados Unidos) | Servidores donde corre la app y registros técnicos |
| MongoDB Atlas | Base de datos principal |
| Neo4j Aura | Relaciones entre personas y lugares para sugerencias de amistad |
| Upstash (Redis) | Datos temporales: límites de peticiones, sesiones cerradas y caché |
| Google (API de Gemini) | Inteligencia artificial para planes y búsqueda por significado (ver sección 5) |
| Amazon S3 y CloudFront (AWS) | Guardan tus fotos en un depósito privado y sirven solo las ya publicadas, sin metadatos de ubicación |
| Amazon Rekognition (AWS) | Revisa cada foto una vez para detectar desnudez, violencia, drogas u odio antes de publicarla; no recibe tu nombre ni datos de tu cuenta |
| Google (Firebase Cloud Messaging) | Envío de notificaciones a tu dispositivo |
| Google (inicio de sesión) | Solo si eliges «Entrar con Google»: confirma tu identidad |
| Open-Meteo | Clima del lugar donde estás; recibe solo coordenadas, sin datos de tu cuenta |
| Brevo (Sendinblue, Francia) | Envío de correos de verificación, recuperación y aviso de tu copia de datos |

Estas comunicaciones a encargados no requieren tu consentimiento.

**Transferencia a Stripe (pagos).** Stripe, Inc. (Estados Unidos) no actúa como encargado nuestro: decide por su cuenta cómo trata los datos de pago, porque la ley le exige verificar quién paga y a quién. Por eso le **transferimos** tus datos solo cuando son necesarios para cumplir lo que nos pides:
- **Si pagas en la app:** tu correo y el monto. Los datos de tu tarjeta los capturas directamente en el formulario de Stripe; Wherever no los recibe.
- **Si tienes un negocio y recibes pagos:** tu identificación, tus datos fiscales y tu cuenta de depósito.

Esta transferencia no requiere tu consentimiento porque es necesaria para el pago que solicitas. Stripe trata estos datos según su propio aviso de privacidad: https://stripe.com/mx/privacy

Fuera de estos casos, solo transferiríamos tus datos a un tercero si una autoridad competente lo exige conforme a la ley.

## 5. Proveedor externo de inteligencia artificial
Para generar planes a partir de lo que escribes y para relacionar negocios por su descripción, Wherever usa un servicio de inteligencia artificial de un tercero: la API de Gemini de Google LLC.

- **Qué se envía:** la frase que escribes para pedir un plan, sin correos, teléfonos, enlaces, nombres de usuario ni coordenadas, que se quitan antes de enviarla; y datos públicos de los negocios (nombre, categoría, precio, distancia aproximada). No se envían tu nombre, tu correo, tu teléfono, tu ubicación exacta ni ningún identificador de tu cuenta.
- **Uso por el proveedor:** el servicio se usa en su modalidad gratuita, en la que Google puede usar lo enviado para mejorar sus productos y personas revisoras pueden leerlo. Por eso te pedimos no escribir datos personales en la frase.
- **Si no quieres usarlo:** puedes armar planes sin la IA; si la IA no está disponible, la app responde con su propio motor de recomendaciones.
- **Edad:** el servicio exige que las personas usuarias tengan 18 años o más, como ya lo exige Wherever.

## 6. Cuánto tiempo guardamos tus datos
| Dato | Plazo |
|---|---|
| Cuenta y perfil | Mientras tengas la cuenta. Si pides eliminarla, se borra a los 30 días; en ese plazo puedes cancelar |
| Sesiones | Hasta que cierras sesión o vence la sesión (máximo 30 días sin uso) |
| Búsquedas recientes | 90 días, o hasta que las borres |
| Enlaces de verificación y recuperación | 24 horas y 1 hora; se borran al usarse |
| Copia de tus datos | 24 horas, o hasta que la descargas |
| Foto original al subirla | Se borra al revisarse; si no se confirma, en 1 día. Queda solo la versión publicada |
| Registro de uso de la IA | 400 días, sin vínculo con tu cuenta después de eliminarla; sirve para controlar el gasto |
| Registros técnicos | 14 días |
| Registro de pagos y de depósitos a negocios | 5 años después del pago, por obligaciones fiscales (Código Fiscal de la Federación), aunque elimines tu cuenta |
| INE y comprobante de domicilio del comerciante | Mientras el negocio esté activo; se borran 90 días después de darlo de baja, salvo que una autoridad los requiera |
| Datos fiscales y cuenta de depósito | Mientras el negocio esté activo y 5 años después, por obligaciones fiscales (Código Fiscal de la Federación) |

## 7. Menores de edad
Wherever es solo para personas de **18 años o más**. Al registrarte pedimos tu fecha de nacimiento para comprobarlo. Si sabemos que una cuenta es de una persona menor, la eliminamos.

## 8. Tus derechos (ARCO) y cómo ejercerlos
Tienes derecho a **acceder** a tus datos, **rectificarlos**, **cancelarlos** (pedir que se borren) y **oponerte** a su uso, así como a **revocar** tu consentimiento.

**Desde la app, sin trámites:**
- **Acceso:** Perfil → Privacidad → «Descargar mis datos». Te llega un enlace por correo con toda tu información en un archivo.
- **Rectificación:** edita tu perfil, tus gustos y tus preferencias en cualquier momento.
- **Cancelación:** Perfil → «Eliminar cuenta», o «Borrar historial» para borrar tu actividad y conservar la cuenta.
- **Oposición y revocación:** apaga los permisos en Perfil → Privacidad.

**Por correo:** escribe a **yahiresendiz34@gmail.com** con tu nombre, el correo de tu cuenta, el derecho que quieres ejercer y una descripción clara de lo que pides. Te pediremos confirmar desde el correo de tu cuenta para verificar tu identidad. Respondemos en un máximo de **20 días hábiles** y, si procede, lo aplicamos dentro de los **15 días hábiles** siguientes.

## 9. Cómo limitar el uso de tus datos
- Apaga la ubicación, el aprendizaje personalizado, el estado de ánimo, los datos sensibles o la búsqueda por teléfono o por nombre de usuario en **Perfil → Privacidad**.
- Apaga las promociones por correo en el mismo lugar.
- Bloquea a una persona desde su solicitud de amistad: dejan de verse en la búsqueda.

## 10. Cookies y tecnologías de rastreo
La app no usa cookies publicitarias ni rastreadores de terceros. Guarda en tu dispositivo solo lo necesario para mantener tu sesión abierta (tokens de acceso), que se borra al cerrar sesión. Al pagar, el componente de Stripe puede recopilar datos del dispositivo para prevenir fraude, según su propio aviso de privacidad.

## 11. Seguridad
Ciframos la comunicación con HTTPS, guardamos las contraseñas con Argon2id y el teléfono solo como huella, limitamos los intentos de acceso y quitamos datos personales de los registros técnicos. Ningún sistema es infalible; si ocurre una vulneración que afecte tus derechos, te avisaremos.

## 12. Cambios a este aviso
Si cambiamos este aviso, la app te mostrará la versión nueva y te pedirá aceptarla antes de seguir. La versión vigente siempre está disponible en la app, en Perfil → Privacidad.

## 13. Autoridad
Si consideras que no atendimos correctamente tus derechos, puedes acudir a la autoridad en materia de protección de datos personales, la **Secretaría Anticorrupción y Buen Gobierno**.

---

## Aviso de privacidad simplificado
*Se muestra en la pantalla de registro, antes de aceptar, con un enlace a este aviso integral.*

Osmar Yahir Reséndiz Vázquez, responsable de Wherever, usa tus datos para crear tu cuenta, recomendarte lugares y planes, conectarte con tus amigos y proteger tu cuenta. Si lo autorizas, también los usamos para personalizar recomendaciones, enviarte promociones y medir el uso de la app; puedes negarte en Perfil → Privacidad sin perder el servicio. Las alergias solo se guardan si activas «Datos sensibles». Para pedir planes usamos la inteligencia artificial de Google, a la que solo enviamos tu frase sin datos personales. No vendemos tus datos. Consulta el aviso de privacidad integral, tus derechos y cómo ejercerlos en la app, en Perfil → Privacidad, o escribe a yahiresendiz34@gmail.com.
