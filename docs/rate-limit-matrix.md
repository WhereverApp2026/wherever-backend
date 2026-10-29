# Rate Limit Matrix — vivo

> Constitución §3, §6. El limitador va antes de Auth.

| Política | Límite | Ventana | Ámbito | Spec |
|---|---|---|---|---|
| Global | 1000 | 1 min | todo | nv-01 |
| Login | 5 | 5 min | `POST /api/v1/auth/*` por IP | módulo auth (posterior) |
| Admin | 200 | 1 min | endpoints admin | módulo admin (posterior) |
| Writes | 10 | concurrentes por usuario | POST/PUT/PATCH/DELETE | nv-01 (configurada) |

429 uniforme con `Retry-After`. Llave `rate:{politica}:{sujeto}` (sujeto = IP o usuario). Con Redis caído el conteo pasa a memoria por instancia (límite efectivo N × límite) y se avisa en el log. Medir antes de fijar valores (principio empírico §2.5).

Aparte del rate limit (son reglas de negocio de su módulo, no políticas globales): bloqueo de login `login_fallos:{correo}` 5 fallos → 15 min con `423` (J1); registro 5 por IP por hora (A1); cuota diaria de planes con IA `cuota:{usuario}:{fecha}` (A4).
