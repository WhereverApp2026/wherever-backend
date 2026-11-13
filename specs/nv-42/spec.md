# NV-42 — Evaluación de imágenes y texto

## Contexto
Antes de publicar, imágenes y texto pasan por servicios gestionados que los clasifican. **Origen**: TEC-31 y hoja A7 del Excel. **Depende de**: NV-26 (modelo), NV-27/28 (tope y límites de la IA, los arma el llamador). La dependencia de NV-18 (caché) se quitó: la moderación no cachea, cada contenido es nuevo.

## Requisitos
1. Paquete `moderacion`: `Moderador.Evaluar(ctx, Contenido) Resultado` devuelve `aprobado`, `cuarentena` o `rechazado` con puntajes por filtro (0 a 100) y, si algo falló, la parte (`Falla`).
2. **Imágenes:** Amazon Rekognition `DetectModerationLabels` (desnudez, violencia, drogas, símbolos de odio) con `MinConfidence` 40. Las demás categorías de Rekognition (traje de baño, alcohol, tabaco, apuestas, gestos) no cuentan: un bar o un balneario no deben caer en cuarentena.
3. **Texto:** clasificación con el modelo de nv-26 en español (drogas, sexual, odio, acoso, spam) más reglas locales: lista de palabras bloqueadas (rechaza sin llamar al modelo) y datos personales (teléfono, correo, enlace, usuario: puntaje 60, cuarentena), con los patrones de `ia.Anonimizar`. **Sin Comprehend** (ver Decisiones).
4. Todo contenido nuevo se publica solo con resultado `aprobado`; si la moderación falla, queda en cuarentena (cierra ante fallos). Si una parte rechaza, rechaza aunque otra haya fallado.

## Diseño
- **Adaptadores** detrás de interfaces: `ImagenModerador` (`moderacion.Rekognition`, AWS SDK v2) y `TextoModerador` (`moderacion.TextoIA` sobre `ia.Modelo`). Las reglas locales las aplica el `Moderador`, antes del modelo.
- **Umbrales:** por debajo de 40 aprueba, desde 40 y por debajo de 80 cuarentena, desde 80 rechaza. Vienen de `FuenteReglas`; en nv-42 son fijas (`moderacion.Fijas(ReglasPorDefecto())`) y nv-43 las guarda en `moderacion_reglas`. Reglas inválidas o fuente caída = cuarentena.
- **Límites:** imagen hasta 5 MB (límite de Rekognition en bytes), texto hasta 4000 caracteres; más = cuarentena sin llamar al proveedor.
- **Sin cableado en `cmd/api`:** no hay todavía contenido que moderar. Lo arma nv-45 (fotos) y la cola de nv-43; el texto debe pasar por la `ia.Guardia` (tope y límites de nv-27/28).
- **Privacidad:** el texto va solo en el turno de usuario y el adaptador de Gemini lo anonimiza; nunca viaja el autor. Los logs solo llevan la parte y el tipo de error, nunca el contenido.

## Costo (piloto, presupuesto US$50 al mes)
- Precio de `DetectModerationLabels` (grupo 2 de imágenes): US$0.001 por imagen hasta 1 millón al mes. Capa gratuita: 1000 imágenes al mes durante el periodo gratuito de la cuenta, si le aplica.
- Infraestructura actual: unos US$41 al mes (nv-12). Margen: unos US$9.
- Volumen estimado del piloto: unas 300 fotos al mes (negocios del piloto y fotos de usuarios) = **US$0.30**. Con 2000 fotos al mes, US$2. Hacen falta 9000 fotos al mes para gastar el margen.
- Se modera **una vez por foto original**, no por cada tamaño (nv-45). nv-45 debe poner un límite de subidas por usuario para que el costo tenga techo; la alarma de presupuesto de nv-13 avisa al correo.
- Texto: Gemini en capa gratuita (costo 0), dentro de las 400 llamadas diarias de nv-28.

## Contratos
Sin endpoints nuevos.

## Configuración e IAM (lo que hace el operador)
- **Sin variables nuevas.** `NewRekognition` recibe la región; en AWS las credenciales salen del rol de la instancia. Ninguna variable obligatoria nueva: no hay riesgo de caída como en nv-17.
- **IAM:** `rekognition:DetectModerationLabels`, solo `us-east-2`, en el boundary (`infra/aws-dev/iam/wherever-dev-boundary.json`), el rol de la instancia (`aws_iam_role_policy.moderacion`) y el deployer (`MedirModeracionEnVivo`, para la prueba en vivo).
- **Antes de fusionar:** nada obligatorio, porque ningún código en ejecución llama a Rekognition todavía.
- **Antes de nv-45 en AWS:** (1) un administrador crea versiones nuevas de `wherever-dev-boundary` y `wherever-dev-deployer-policy` en la consola de IAM con los JSON del repo; (2) `terraform apply` en `infra/aws-dev` con el perfil `wherever-dev` (solo agrega la política en línea del rol); (3) recomendado: política de exclusión de servicios de IA de AWS Organizations para que AWS no use las imágenes para mejorar Rekognition.

## Tests
- `Unit/Umbrales` (`internal/moderacion/umbrales_test.go`): bordes 0, 39.99, 40, 79.99, 80 y 100; reglas inválidas; reglas inválidas o fuente caída = cuarentena sin llamar al modelo; contenido vacío.
- `Unit/Texto` (`texto_test.go`) con `iatest.Modelo`: limpio, venta de drogas, odio, spam dudoso, teléfono, texto con instrucciones al modelo, palabra bloqueada sin llamar al modelo, palabra completa, modelo caído (sin el texto en los logs), salida inválida dos veces, texto largo y sin adaptador.
- `Unit/Imagen` (`imagen_test.go`): el adaptador real contra `internal/testutil/rekognitiontest` (servidor local que habla JSON 1.1 de AWS): limpia (firmada con SigV4 y `MinConfidence` 40), explícita, dudosa por su raíz, alcohol y traje de baño no cuentan, servicio caído (500, `ThrottlingException`, `AccessDeniedException`), imagen grande sin salir, rechazo gana a una falla del texto.
- Son Unit y no Integration: el paquete no usa bases; los proveedores se prueban con dobles locales, como `fcmtest`.
- En vivo, una vez (`vivo_test.go`, `MODERACION_LIVE=1`): 5 imágenes lisas contra Rekognition y 3 textos contra Gemini, con latencia. Pendiente: el deployer no tiene el permiso hasta aplicar la política nueva.

## Criterios
- Nada se publica sin resultado aprobado (`Resultado.Publicable`).
- Si la moderación falla, el contenido queda en cuarentena.

## Decisiones
- **Sin Comprehend:** aunque `DetectPiiEntities` soporta español, lo que la spec le pedía (teléfonos y correos) ya lo detectan las reglas de `ia.Anonimizar`, sin salir de la API. Ahorra un permiso, una llamada por texto y su costo; los nombres propios siguen sin detectarse (igual que en nv-28).
- **AWS SDK v2** (Apache 2.0, de AWS, con IMDSv2, firma y reintentos) en lugar de un cliente propio: escribir SigV4 y la lectura de credenciales del rol a mano es más riesgo que la dependencia. Reintenta una vez.
- **Aviso de privacidad:** nv-42 no envía datos de nadie. nv-45 (el primero que modera contenido de usuarios) agrega Rekognition y la moderación de texto al aviso y sube `AVISO_VERSION`.

## Límites
- Los falsos positivos y negativos existen; la cola humana (nv-43) es la red de seguridad.
- Sin moderación de video ni audio.
- Rekognition es una API en la nube: la imagen sale de la API hacia AWS (misma región).
- Bajar el umbral de cuarentena por debajo de 40 no ve etiquetas menores: `MinConfidence` es 40.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-13), implementada en la rama `nv-42`. Ola 6: imágenes y moderación.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** confirmar la estimación de volumen (300 fotos al mes) y que el límite de subidas va en nv-45.
- **Detalle:** evidencia en `specs/nv-42/checklists/gates.md` + `Memoria.md`.
