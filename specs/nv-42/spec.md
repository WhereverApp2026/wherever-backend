# NV-42 — Evaluación de imágenes y texto

## Contexto
Antes de publicar, imágenes y texto pasan por servicios gestionados que los clasifican. **Origen**: TEC-31 y hoja A7 del Excel. **Depende de**: NV-26, NV-18.

## Requisitos
1. Paquete `moderacion.Evaluar(ctx, Contenido)` que devuelve `aprobado`, `cuarentena` o `rechazado` con puntajes por filtro.
2. **Imágenes:** Amazon Rekognition `DetectModerationLabels` (desnudez, violencia, drogas, símbolos de odio) con umbrales configurables.
3. **Texto:** clasificación con el modelo de nv-26 en español (venta de drogas, contenido sexual, odio, acoso, spam) más una lista de palabras bloqueadas; Comprehend solo se usa para detectar datos personales (teléfonos, correos), que sí soporta español.
4. Todo contenido nuevo se publica solo con resultado `aprobado`; si la moderación falla, queda en cuarentena (cierra ante fallos).

## Diseño
- **Adaptadores** detrás de una interfaz: `ImagenModerador` (Rekognition) y `TextoModerador` (modelo + reglas).
- **Umbrales:** por debajo de 40 aprueba, entre 40 y 80 cuarentena, desde 80 rechaza (ajustables en `moderacion_reglas`).
- **Costo:** Rekognition tiene capa gratuita limitada; el uso se mide.
- **Privacidad:** el texto enviado al modelo va sin identificadores del autor.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Umbrales 8`: bordes de aprobar, cuarentena y rechazar.
- `Integration/Texto 10` con servidor local del modelo: texto limpio, venta de drogas, odio, spam, teléfono, texto con instrucciones al modelo.
- `Integration/Imagen 6` contra servidor local de Rekognition: limpia, explícita, dudosa, servicio caído (cuarentena).
- En vivo, una vez: 5 imágenes de prueba inofensivas y 3 textos para medir latencia y resultados.

## Criterios
- Nada se publica sin resultado aprobado.
- Si la moderación falla, el contenido queda en cuarentena.

## Límites
- Los falsos positivos y negativos existen; la cola humana es la red de seguridad.
- Sin moderación de video ni audio.
- Rekognition es una API en la nube: la imagen sale de la API hacia AWS (misma región).

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-42/checklists/gates.md` + `Memoria.md`.
