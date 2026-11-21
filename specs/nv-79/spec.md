# NV-79 — Insignias, puntos y nivel como en la maqueta

## Contexto
nv-78 creó 8 insignias con nombres y metas propios. La maqueta (`83_Insignias`, `81_Perfil`, `26_PerfilAmigo`) pide otro catálogo, un **nivel con puntos** («Explorador 3 · 340 de 500 puntos para ser Anfitrión», «Nv. 4») y una **racha** en semanas. Hoy `usuarios.puntos`, `nivel` y `racha_semanas` existen pero solo cambian al registrarse (50 puntos, «Explorador»). **Origen**: revisión de la maqueta contra la API (2026-11-21). **Depende de**: NV-78 (módulo `insignias`), NV-62 (check-ins y reseñas), NV-63 (postales), NV-37 (grupos), NV-83 (me gusta, para «Querido»).

## Requisitos
1. El catálogo de insignias es el de la maqueta, con ganadas y por desbloquear.
2. La persona acumula puntos por lo que hace y sube de nivel; ve cuánto le falta para el siguiente.
3. La racha cuenta semanas seguidas con al menos una visita verificada.
4. Los puntos no se pueden inflar repitiendo una acción ni borrando y creando de nuevo.

## Diseño
- **Catálogo (reemplaza el de nv-78; los ids viejos quedan guardados pero no se muestran):**

  | id | Nombre | Meta |
  |---|---|---|
  | `catador_cafe` | Catador de café | check-in válido en 5 cafeterías distintas |
  | `resenista` | Reseñista | 1 reseña publicada (visita verificada) |
  | `anfitrion_grupo` | Anfitrión de grupo | organizar 1 grupo |
  | `viajero_pueblo` | Viajero del pueblo | 10 lugares distintos con check-in |
  | `cronista` | Cronista | 10 postales listas |
  | `racha_3` | Racha de 3 semanas | racha ≥ 3 |
  | `sello_oro` | Sello de oro | 20 reseñas publicadas |
  | `querido` | Querido | 50 me gusta recibidos (nv-83; sin nv-83 queda bloqueada) |
  | `explorador_total` | Explorador total | 30 lugares distintos con check-in |
- **Puntos con bitácora idempotente:** colección `puntos_movimientos` `{usuario_id, motivo, ref_id, puntos, creado_en}` con índice único `ux_movimiento` `{usuario_id, motivo, ref_id}`. Motivos y valores: `registro` 50 (existente), `checkin` 20 por check-in válido (un lugar cuenta una vez por día, como `ux_checkin_dia`), `resena` 30 por reseña publicada, `postal` 10 por postal lista, `plan` 5 por plan creado (máx. 5 por semana), `insignia` 50 por insignia ganada. Borrar algo **no** resta (evita castigar la limpieza) y crearlo de nuevo no suma otra vez (mismo `ref_id`). `usuarios.puntos` se actualiza con `$inc` solo cuando la inserción del movimiento no es duplicada.
- **Cuándo se otorgan:** al pedir `GET /usuarios/yo/insignias` o `GET /usuarios/yo` se concilian los movimientos que falten (mismos conteos de nv-78 con ids de referencia), sin eventos en cada módulo. Un trabajo nocturno (nv-05) concilia a quienes no abrieron la app.
- **Niveles (tabla versionada en el código):** Nv. 1–3 «Explorador» (0, 150, 300), Nv. 4–5 «Anfitrión» (500, 800), Nv. 6–7 «Guía local» (1 200, 1 700), Nv. 8+ «Leyenda» (2 300 y +700 por nivel). `usuarios.nivel` guarda el nombre y `usuarios.nivel_numero` el número.
- **Racha:** semanas ISO (America/Mexico_City) consecutivas, hasta la actual, con ≥ 1 check-in válido; si la semana actual aún no tiene, cuenta hasta la anterior. Se guarda en `usuarios.racha_semanas`.
- **Respuesta de `GET /usuarios/yo/insignias`:** agrega `nivel{numero, nombre, puntos, siguiente{numero, nombre, puntos}?}` y `racha_semanas`. `GET /personas/{id}` muestra `nivel{numero, nombre}` (ver nv-81).
- **ARCO:** `puntos_movimientos` se exporta y se borra con la cuenta.

## Contratos
| Método | Ruta | Cambio |
|---|---|---|
| GET | `/api/v1/usuarios/yo/insignias` | catálogo nuevo; `nivel` y `racha_semanas` |
| GET | `/api/v1/usuarios/yo` | `puntos`, `nivel`, `nivel_numero` y `racha_semanas` ya conciliados |

Sin rutas nuevas.

## Tests
- Unit: tabla de niveles (bordes 149/150, 499/500), racha con semanas vacías y año nuevo, catálogo.
- Integration: puntos por cada motivo; repetir o borrar y recrear no suma; `$inc` solo con movimiento nuevo; conciliación concurrente sin duplicar; nivel y siguiente; racha; «Querido» bloqueada sin nv-83.
- Security: los puntos de otra persona no se pueden mover; nada de `puntos_movimientos` en `/personas/{id}`.
- Database: `ux_movimiento` único. Contract: insignias con nivel; exportación con `puntos_movimientos`.

## Criterios
- Los puntos solo suben por acciones verificables y una sola vez por acción.
- El catálogo, el nivel y la racha coinciden con `83_Insignias`.

## Límites
- Sin canjear puntos, sin tabla de líderes, sin insignias por temporada.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) valores de puntos y tabla de niveles (la maqueta muestra «Nv. 4 · Explorador» y «340 de 500 para Anfitrión», que no cuadran entre sí); (2) borrar no resta puntos; (3) los ids de nv-78 dejan de mostrarse; (4) conciliar al pedir más un trabajo nocturno, sin eventos en cada módulo.
- **Detalle:** `specs/nv-79/checklists/gates.md` al ejecutar.
