# NV-41 — Envío de push con Firebase

## Contexto
Los avisos llegan al teléfono respetando preferencias y horario silencioso, sin bloquear nada. **Origen**: US-162 y hoja A6 del Excel. **Depende de**: NV-40.

## Requisitos
1. Todo evento crea la notificación en el centro y, si corresponde, envía push por FCM.
2. El push respeta las preferencias del usuario por tipo y el **horario silencioso**; en silencio la notificación queda en el centro y no suena.
3. Tokens inválidos (`UNREGISTERED`) se borran solos.
4. Envío en segundo plano con reintentos; un fallo de FCM nunca cambia la respuesta de la acción original.
5. Sin contenido sensible en el push: título y texto genéricos.

## Diseño
- **FCM HTTP v1** con una cuenta de servicio de Firebase cuyo JSON se guarda en SSM (`/wherever/dev/fcm-service-account`); la carga la hace el equipo, sin pasarla por el chat.
- **Cola** acotada con trabajadores, igual que el correo (nv-06): `Enqueue` nunca bloquea; cola llena descarta y cuenta.
- **Horario silencioso:** en preferencias, en la zona del usuario (México por defecto).
- **Interfaz** `notificaciones.Emitir(ctx, destinatario, tipo, datos)`: la usan nv-36 y nv-37.
- **Logs:** tipo y resultado; nunca token ni cuerpo.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Preferencias 8`: tipo apagado, silencio dentro y fuera del horario, cambio de día.
- `Integration/FCM 8` contra servidor local que habla la API v1: envío, token inválido que se borra, 5xx con reintento, FCM caído sin romper la acción.
- `Security/Notificaciones 4`: ningún token ni contenido sensible en logs; sexto dispositivo rechazado.
- En vivo, una vez: push real a un teléfono de prueba.

## Criterios
- Un fallo de FCM no afecta la acción que originó el aviso.
- En horario silencioso no suena.

## Límites
- Sin push a iOS.
- Sin notificaciones programadas (recordatorios llegan después).
- La entrega no está garantizada: FCM es de mejor esfuerzo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 5: grupos y notificaciones, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-41/checklists/gates.md` + `Memoria.md`.
