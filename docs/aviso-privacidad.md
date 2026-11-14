# Aviso de privacidad — terceros que tratan datos (vivo)

> Texto canónico de la sección de proveedores externos del aviso de privacidad. La app lo muestra y la persona lo acepta con `POST /api/v1/usuarios/yo/consentimientos` (nv-19). Cambiar esta sección obliga a subir `AVISO_VERSION`: con eso `privacidad.consentimiento_vigente` pasa a falso y la app vuelve a pedir la aceptación.

## Proveedor externo de inteligencia artificial
Para generar planes a partir de lo que escribes y para relacionar negocios por su descripción, Wherever usa un servicio de inteligencia artificial de un tercero: la API de Gemini de Google LLC (nv-26, nv-50).

- **Qué se envía:** la frase que escribes para pedir un plan, sin correos, teléfonos, enlaces, nombres de usuario ni coordenadas, que se quitan antes de enviarla; y datos públicos de los negocios (nombre, categoría, precio, distancia aproximada). No se envían tu nombre, tu correo, tu teléfono, tu ubicación exacta ni ningún identificador de tu cuenta.
- **Uso por el proveedor:** el servicio se usa en su modalidad gratuita, en la que Google puede usar lo enviado para mejorar sus productos y personas revisoras pueden leerlo. Por eso te pedimos no escribir datos personales en la frase.
- **Si no quieres usarlo:** puedes armar planes sin la IA; si la IA no está disponible, la app responde con su propio motor de recomendaciones.
- **Edad:** el servicio exige que las personas usuarias tengan 18 años o más, como ya lo exige Wherever.

## Otros encargados
- **Correo:** proveedor por elegir (nv-06). Se agrega aquí al elegirlo.
- **Almacenamiento de fotos:** Amazon S3 de Amazon Web Services guarda las fotos que subes en un depósito privado; el original se borra al revisarse y, si no se confirma, en un día (nv-44, nv-45).
- **Entrega de fotos:** Amazon CloudFront de Amazon Web Services sirve solo las fotos publicadas, ya sin metadatos como la ubicación GPS (nv-44, nv-45).
- **Revisión automática de fotos:** Amazon Rekognition de Amazon Web Services analiza cada foto una vez para detectar desnudez, violencia, drogas u odio antes de publicarla; no recibe tu nombre ni datos de tu cuenta (nv-42, nv-45).
- **Pagos:** Stripe Payments México cobra el anticipo de las reservas. Los datos de tu tarjeta los capturas en el formulario de Stripe dentro de la app y van directo a Stripe; Wherever nunca recibe el número ni el código de seguridad. Wherever guarda solo el monto, la fecha, el estado, el concepto, la marca y los últimos 4 dígitos de la tarjeta, y conserva esos registros 5 años por obligación fiscal (Código Fiscal de la Federación), aunque borres tu cuenta (nv-60).
- **Depósitos a negocios:** para recibir sus anticipos, cada negocio aliado se registra en Stripe Connect, que verifica su identidad y recibe su cuenta bancaria (CLABE); Wherever guarda la CLABE cifrada en el expediente del negocio (nv-55), muestra solo los últimos 4 dígitos y conserva el registro de cada depósito semanal por 5 años (nv-60).
