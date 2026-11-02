# Rate Limit Matrix — vivo

> Constitución §3, §6. El limitador va antes de Auth.

| Política | Límite | Ventana | Ámbito | Spec |
|---|---|---|---|---|
| Global | 1000 | 1 min | todo | nv-01 |
| Login | 5 | 5 min | `POST /api/v1/auth/*` por IP | módulo auth (posterior) |
| Admin | 200 | 1 min | endpoints admin | módulo admin (posterior) |
| Writes | 10 | concurrentes por usuario | POST/PUT/PATCH/DELETE | nv-01 (por IP hasta que exista auth) |

429 uniforme con `Retry-After`. Global = ventana fija en Redis (`INCR` + `EXPIRE NX`); Writes = contador de concurrencia con TTL de 2 min por si una instancia muere a media petición. Valores por `RATE_LIMIT_GLOBAL` y `RATE_LIMIT_WRITES`; en prod no se pueden subir. Llave `rate:{politica}:{sujeto}` (sujeto = IP o usuario). Con Redis caído el conteo pasa a memoria por instancia (límite efectivo N × límite), se avisa una vez en el log y no se reintenta Redis durante 5 s para no frenar cada petición. Medir antes de fijar valores (principio empírico §2.5).

Aparte del rate limit (son reglas de negocio de su módulo, no políticas globales): bloqueo de login `login_fallos:{correo}` 5 fallos → 15 min con `423` (J1); registro 5 por IP por hora (A1); cuota diaria de planes con IA `cuota:{usuario}:{fecha}` (A4).
