# Rate Limit Matrix — vivo

> Constitución §3, §6. El limitador va antes de Auth. La IP sale de `X-Forwarded-For` a `TRUSTED_PROXY_HOPS` del final y solo de un proxy de confianza; las entradas más a la izquierda se ignoran (nv-12).

| Política | Límite | Ventana | Ámbito | Spec |
|---|---|---|---|---|
| Global | 1000 | 1 min | todo | nv-01 |
| Login | 5 | 5 min | `POST /api/v1/auth/login` por IP (`rate:login:{ip}`) | nv-04 |
| Registro | 5 | 1 h | `POST /api/v1/usuarios` por IP (`rate:registro:{ip}`, hoja A1) | nv-05 |
| Correo disponible | 20 | 1 min | `GET /api/v1/usuarios/correo-disponible` por IP; provisional | nv-05 |
| Descarga de exportación | 10 | 1 min | `POST /api/v1/usuarios/exportaciones/descargar` por IP; provisional | nv-05 |
| Auth enlaces | 10 | 1 min | `refresh`, `recuperar`, `restablecer`, `verificar-correo`, cada una con su llave (`rate:auth_{ruta}:{ip}`) por IP; valor provisional, se mide antes de endurecer | nv-04 |
| Admin | 200 | 1 min | `POST /api/v1/admin/negocios/importar` y `/api/v1/admin/moderacion/*` por IP, en la misma cuenta (`rate:admin:{ip}`) | nv-20, nv-43 |
| Writes | 10 | concurrentes por usuario | POST/PUT/PATCH/DELETE | nv-01 (por IP hasta que exista auth) |
| IA por usuario | 10 generaciones (5 planes: interpretar + itinerario) | 1 h | `POST /api/v1/planes/generar*` por usuario (`rate:ia_usuario:{usuario}`, ventana fija desde la primera); al pasarlo responde el motor de reglas (`fuente: reglas`, nv-33); un acierto de caché no cuenta; sin Redis también | nv-28, nv-33 |
| Buscar personas | 30 | 1 h | `GET /api/v1/personas/buscar` por usuario (`rate:personas_buscar:{usuario}`, ventana fija) | nv-35 |
| Solicitudes de amistad | 20 | 1 h | `POST /api/v1/amistades` por usuario (`rate:amistades_solicitud:{usuario}`, ventana fija); cuenta toda solicitud, también las rechazadas con 404, 409 o 422 | nv-36 |
| Invitaciones a grupos | 30 | 1 h | `POST /api/v1/grupos`, `POST /api/v1/grupos/{id}/integrantes` y `POST /api/v1/planes/{id}/invitaciones` por usuario (`rate:grupos_invitacion:{usuario}`, ventana fija); cuenta cada petición, también las rechazadas | nv-37 |
| Plan compartido | 30 | 1 min | `POST /api/v1/planes/compartidos` por IP (`rate:plan_compartido:{ip}`, ventana fija); vista pública sin sesión | nv-38 |
| Unirse por enlace | 30 | 1 h | `POST /api/v1/grupos/unirse` por usuario (`rate:grupos_unirse:{usuario}`, ventana fija) | nv-38 |
| Acciones sin conexión | 30 lotes | 1 min | `POST /api/v1/sync/pendientes` por usuario (`rate:sync_pendientes:{usuario}`, ventana fija); un lote de hasta 100 acciones cuesta los mismos 3 comandos | nv-47 |
| Búsqueda semántica | 30 | 1 min | `GET /api/v1/lugares/semantica` por usuario o, sin token, por IP (`rate:semantica:{sujeto}`, ventana fija); además, cada frase sin caché cuenta un texto del límite de embeddings | nv-51 |
| IA global | 12 llamadas / 400 llamadas | 1 min / 1 día (hora del Pacífico) | llamadas a Gemini de toda la app (`rate:ia_min`, `rate:ia_dia`); al llegar, responde el motor de reglas sin 429. 80 % de lo que AI Studio da al proyecto con `gemini-3.5-flash-lite` (15 por minuto, 500 por día); sin Redis no se llama al proveedor | nv-28 |
| Embeddings global | 80 textos / 800 textos | 1 min / 1 día (hora del Pacífico) | textos vectorizados con `gemini-embedding-2` de toda la app (`rate:ia_emb_min`, `rate:ia_emb_dia`); cada texto de un lote cuenta uno. Cuota aparte de la del modelo de texto. AI Studio no publica la gratuita de este modelo: 80 % de la de `gemini-embedding-001` (100 por minuto, 1000 por día), a confirmar en AI Studio. Al llegar o sin Redis no se vectoriza y responde el motor de reglas | nv-50 |

429 uniforme con `Retry-After`. Global = ventana fija en Redis (`INCR` + `EXPIRE NX`); Writes = contador de concurrencia con TTL de 2 min por si una instancia muere a media petición. Valores por `RATE_LIMIT_GLOBAL` y `RATE_LIMIT_WRITES`; en prod no se pueden subir. Llave `rate:{politica}:{sujeto}` (sujeto = IP o usuario). Con Redis caído el conteo pasa a memoria por instancia (límite efectivo N × límite), se avisa una vez en el log y no se reintenta Redis durante 5 s para no frenar cada petición. Medir antes de fijar valores (principio empírico §2.5).

Las políticas por ruta van en la ruta, antes de `RequireAuth`; el pipeline global no cambia. Aparte del rate limit (son reglas de negocio de su módulo, no políticas globales): bloqueo de login `login_fallos:{correo}` 5 fallos → 15 min con `423`, fail-closed (J1, nv-04); reenvío de enlaces 1 cada 60 s con `429` y `Retry-After` (J1, nv-04, se cuenta en `tokens_un_uso`, no en Redis); "Descargar mis datos" 1 por día con `429` (A1, nv-05, se cuenta en `exportaciones`); registro 5 por IP por hora (A1); cuota diaria de planes con IA `cuota:{usuario}:{fecha}` (A4).
