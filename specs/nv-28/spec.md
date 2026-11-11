# NV-28 — Límite por usuario y protección de datos en la IA

## Contexto
Un usuario no puede agotar la IA y nada personal viaja al modelo. **Origen**: US-159, US-190. **Depende de**: NV-27.

## Requisitos
1. **Límite por usuario** y por hora (`IA_LIMITE_HORA`, 5 generaciones por defecto, hasta 10 llamadas con el reintento): `ia.ErrLimiteUsuario` como `*ia.LimiteUsuarioError` con `Regreso` (hora exacta) y `RetryAfter()` (segundos) para el 429 del endpoint; el rechazo queda en `usos_ia` con resultado `limite_usuario`. Un acierto de caché no cuenta. Sin usuario en el contexto, todas las generaciones comparten la cubeta `sin_usuario`.
2. **Límite global por debajo de la capa gratuita de Gemini:** `IA_LIMITE_MIN` (llamadas por minuto, 12 por defecto, máximo 15) e `IA_LIMITE_DIA` (llamadas por día, 400 por defecto, máximo 500), compartidos por las dos instancias en Redis. Cada uno es el 80 % del límite que AI Studio muestra para el proyecto con `gemini-3.5-flash-lite` (2026-11-09: 15 por minuto, 500 por día y 250 000 tokens por minuto). Los tokens no limitan: cada llamada usa unos 215. Si cambia el modelo o el límite, se recalculan; el día se cuenta en hora del Pacífico, como el del proveedor. Al llegar a cualquiera de los dos, `ia.ErrLimiteGlobal` y responde el motor de reglas sin llamar al proveedor (sin 429).
3. **Defensa contra inyección:** el texto del usuario va solo en el turno de usuario, nunca en las instrucciones (`ia.Peticion` los separa y el adaptador pone las instrucciones solo en `systemInstruction`); el modelo solo puede elegir ids de una lista que la API le entrega: las salidas validan con `ia.SoloDeLaLista` desde su `Validar`, y una salida con un id ajeno se descarta (`ErrRespuestaInvalida`).
4. **Sin datos personales hacia el modelo:** sin nombre, correo, teléfono ni coordenadas exactas (se envían distancias y zona aproximada en `Contexto`). La frase pasa por `ia.Anonimizar` (nv-26) y nunca viajan ids de usuario: `ia.ConUsuario` solo llega a `usos_ia`. La capa gratuita de Gemini puede usar lo enviado para mejorar sus productos.

## Diseño
- **Integración con `ia.Guardia` (nv-27):** el límite global va en `Guardia.Generar`, después del tope de gasto, así cuenta cada llamada al proveedor, también el reintento de `GenerarValidado`. El límite por usuario va en `Guardia.GenerarValidado`, después de la caché: cuenta generaciones, no llamadas. `GuardiaConfig` suma `Limites` y `Limitador` (obligatorio). Código en `internal/ia/limite.go`.
- **Redis:** el límite por usuario y el global siguen cerrados ante fallos (`ia.ErrLimiteDesconocido`: sin Redis no se llama al proveedor y responde el motor de reglas); la caché no.
- **Llaves (ventanas fijas):** `rate:ia_usuario:{usuario}` (1 h desde la primera generación), `rate:ia_min:{AAAAMMDDHHMM UTC}` (1 min) y `rate:ia_dia:{AAAA-MM-DD Pacífico}` (25 h). La vida se pone solo al nacer la llave (`SET NX` con TTL). Una llamada rechazada también cuenta: el límite queda del lado seguro.
- **Presupuesto de Redis:** `LimitadorRedis.Tomar` hace por ventana `SET NX`, `INCR` y `PTTL` en un solo Pipeline: 3 comandos por generación (usuario) y 6 por llamada (global), fijados en `TestPresupuestoRedis_LimitesIA`. Con 50 generaciones al día es cerca del 2 % de la cuota de Upstash; con el tope diario lleno (200 generaciones, 400 llamadas) son 3000 al día, unos 18 %, unos 58 % en total con nv-11 y nv-27, todavía dentro de la cuota.
- **Configuración:** `IA_LIMITE_HORA` (1-50), `IA_LIMITE_MIN` (1-15) e `IA_LIMITE_DIA` (1-500) en `internal/platform/config` y `configs/.env.example`. El máximo es el límite del proveedor; con la facturación activa se sube en el código.
- **Cableado:** ninguno todavía, como nv-27. El endpoint de nv-31/nv-33 arma la guardia con `ia.NewLimitadorRedis` y traduce `LimiteUsuarioError` a 429 con `Retry-After`.

## Contratos
Sin endpoints nuevos.

## Tests
- Unit `internal/ia/limite_test.go`: sexta generación con hora de regreso, otro usuario, cubeta sin usuario, acierto de caché no cuenta, global por minuto y por día, el reintento cuenta, Redis caído cierra, llaves del día del Pacífico; `esquema_test.go`: `SoloDeLaLista`; config: valores por defecto y rangos.
- `Integration/Límite 8` con Redis real (`tests/integration/ia_limite_test.go`): sexta generación, hora de regreso desde el TTL, otro usuario no se ve afectado, límite global por minuto y por día del Pacífico, Redis caído (cierra), rechazo registrado en `usos_ia`, llaves con TTL; además `TestPresupuestoRedis_LimitesIA`.
- `Security/IA 4` (`tests/security/ia_security_test.go`, adaptador real de Gemini contra un servidor local): texto con instrucciones («ignora lo anterior») no cambia las reglas y un id ajeno se descarta; el texto del usuario nunca aparece en las instrucciones; ningún dato personal ni id de usuario hacia el modelo; ni `usos_ia` ni la caché guardan la frase.

## Criterios
- Ninguna llamada lleva datos personales.
- Sin Redis, ninguna llamada al proveedor.

## Límites
- Sin límites propios más allá de los ya anotados.
- La anonimización no reconoce nombres propios (nv-26); la app pide no escribirlos.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11): implementada en la rama `nv-28` (PR contra `nv-27`). Unit, lint, critic, SDD y endpoints en verde; Integration y Security en el CI del PR.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** límites integrados en `ia.Guardia`; generaciones sin usuario en una cubeta común; los máximos de configuración son los del proveedor.
- **Detalle:** la evidencia irá en `specs/nv-28/checklists/gates.md` + `Memoria.md`.
