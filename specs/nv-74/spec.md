# NV-74 — Estadísticas y reporte mensual del negocio

## Contexto
El dueño ve hoy el resumen del día en el panel (nv-56, P-47), pero no su historia: visitas por día y hora, de dónde llegan, cómo va contra su categoría y un reporte mensual para su socio o contador (US-144). nv-56 difirió las tres piezas «cuando existan reservas, check-ins y cupones»; ya existen. **Origen**: hoja «A5 Negocios aliados» del Excel `Wherever_Endpoints_CRUD_v3.xlsx` (colección `estadisticas_diarias`, dos filas de endpoints y el filtro «Estadísticas por rango»). **Depende de**: NV-04/NV-05 (sesión, `RequireActive`), NV-11 (presupuesto de Redis), NV-54 (`favoritos`), NV-56 (propiedad, `negocios.Observador`, panel), NV-59 (`reservas`), NV-61 (`cupones`, `cupones_metricas`), NV-62 (`checkins`, `resenas`) y NV-66 (`internal/vistas`, `historial_vistas`).

## Requisitos
1. `GET /negocios/{id}/estadisticas` da, para un rango, las cifras por día o semana, la comparación contra el periodo anterior y una frase sencilla; las avanzadas solo con plan premium.
2. `GET /negocios/{id}/reportes/mensual?mes=` descarga un PDF del mes.
3. Las cifras son anónimas: ningún documento de estadísticas lleva `usuario_id`, y borrar una cuenta no cambia los días ya cerrados.
4. El panel (nv-56) no se duplica: el panel es «hoy en vivo»; las estadísticas son «rango cerrado».

## Diseño
### Colección `estadisticas_diarias` (Excel A5 §3, ajustada)
- Un documento por negocio y día local (`America/Mexico_City`): `negocio_id`, `fecha` (día, guardado como `AAAA-MM-DD`, como `cupones_metricas.dia`), `vistas_ficha`, `reservas`, `llegadas`, `cupones_canjeados`, `favoritos_nuevos`, `origen {busqueda, mapa, inicio, plan_ia, cupon}`, `por_hora [24]` (llegadas por hora), `calificacion_dia?`, `resenas_dia`, `aspectos_suma` (suma de los cinco aspectos de las reseñas del día), `categoria` (la del negocio, para el resumen de categoría) y `actualizado_en`. Índice `ux_negocio_fecha` `{negocio_id: 1, fecha: -1}` único, en `scripts/indexes` y `/ready`. Sin TTL: guarda 400 días y el trabajo nocturno borra lo más viejo.
- **Drift con el Excel:** `origen` usa los valores de `historial_vistas` (nv-66: `busqueda`, `mapa`, `inicio`, `plan_ia`, `cupon`); el Excel dice `favorito` en lugar de `mapa`/`inicio`. Se agregan `favoritos_nuevos`, `resenas_dia` y `aspectos_suma` (para promediar semanas y aspectos con el peso de cada día, sin volver a leer `resenas`) y `categoria`. `fecha` va como texto de día local, no `date`.

### Cómo se llena
- **Vistas, en vivo:** `historial_vistas` guarda solo la última vista por persona (nv-66) y se borra con ARCO, así que no sirve para contar. Cuando `vistas` sí escribe (pasó la ventana de 10 min), hace además un `upsert` con `$inc {vistas_ficha: 1, "origen.<o>": 1}` en el día local. Una escritura más de MongoDB por vista contada, cero comandos de Redis. Se inyecta como interfaz `vistas.Contador` para no acoplar paquetes.
- **El resto, por trabajo cada hora:** recalcula hoy y ayer de cada negocio con actividad con agregaciones idempotentes (`$set`, nunca `$inc`): `reservas` (creadas ese día, sin `bloqueada` ni `expirada`), `llegadas` y `por_hora` (`checkins` válidos), `cupones_canjeados` (`cupones_usuario.canjeado_en` del negocio en el día local, índice `ix_canjes_negocio`; `cupones_metricas.dia` es día UTC y no cuadra con el día local), `favoritos_nuevos` (`favoritos` tipo lugar con `creado_en` ese día) y `calificacion_dia`/`resenas_dia` (`resenas` publicadas). Solo MongoDB, sin candado de Redis (dos instancias escriben lo mismo, como el barrido de nv-59). Días anteriores a ayer quedan congelados: si una persona borra su cuenta, sus reservas se van pero el total del día ya cerrado se queda (es anónimo).
- **Primera corrida:** rellena los últimos 90 días de los negocios aprobados o pausados, de a 50 negocios por hora para no cargar MongoDB; las vistas anteriores a esta spec quedan en 0. Qué negocio ya se rellenó lo marca `estadisticas_relleno` (`{_id: negocio_id, hecho_en}`, sin índices propios).
- **Qué negocios toca cada hora:** los que tienen reservas, check-ins, canjes o favoritos creados desde ayer, o reseñas con `actualizado_en` desde ayer (para ver las que se publican después de moderarse). Escribe hoy y ayer completos (en cero si ya no hay nada) para que una cifra que baja también se corrija.
- **Baja del negocio:** `negocios.Observador.DadoDeBaja` borra sus `estadisticas_diarias` y su marca de relleno.
- **Nocturno:** corre como `privacidad.Tarea` (`estadisticas_negocio`, con el candado del trabajo nocturno): resume el mes de ayer y el anterior en `estadisticas_categoria` y borra los días de más de 400 días.

### Estadísticas
- **Quién:** JWT `comerciante` + `RequireActive`, dueño del negocio (si no, 404 como en nv-56).
- **Query:** `desde`, `hasta` (`AAAA-MM-DD`, locales; por omisión los últimos 30 días hasta ayer), `agrupar` (`dia` por omisión o `semana`, lunes a domingo). Rango máximo 90 días (como cupones, nv-61); `desde > hasta`, fechas futuras o inválidas = 422.
- **Respuesta:** `{ desde, hasta, agrupar, serie: [{ inicio, vistas_ficha, reservas, llegadas, cupones_canjeados, favoritos_nuevos, calificacion }], totales: {…}, anterior: {…totales del periodo anterior de la misma longitud}, cambio: {campo: porcentaje | null}, frase, avanzadas: {por_hora: [24], origen: {…}, aspectos: [{aspecto, tuyo, categoria}]} | null, premium_requerido: bool, actualizado_en }`.
- **Serie por semana:** cada punto empieza en su lunes, salvo el primero, que empieza en `desde` (`inicio` siempre cae dentro del rango).
- **Avanzadas solo premium** (`negocios.plan = premium`): `por_hora` (suma del rango), `origen` y `aspectos` contra la categoría. Sin premium, `avanzadas: null` y `premium_requerido: true` (200, no 403: la app muestra el candado).
- **Aspectos contra la categoría:** promedio de los cinco aspectos de nv-62 en el rango, del negocio y de su categoría (todo el país: `negocios` no guarda ciudad). La cifra de la categoría sale de `estadisticas_categoria` (`categoria`, `mes`, `aspectos`, `calificacion`, `resenas`, `negocios`; índice `ux_categoria_mes` único), que el trabajo nocturno calcula una vez al día desde `estadisticas_diarias`; se compara con el mes de `hasta`. Si la categoría tiene menos de 5 negocios con reseñas en ese mes, `categoria: null`: con menos, el promedio deja adivinar a un competidor.
- **Frase sin IA:** plantilla por reglas con el cambio más grande (p. ej. «Tus visitas subieron 18 % frente a las 4 semanas anteriores»; «Aún no hay datos suficientes» con menos de 7 días con actividad). El consejo con IA es de nv-75.
- **Hoy:** el rango termina ayer por omisión; si `hasta` es hoy, el día puede ir hasta 1 h atrasado (`actualizado_en` lo dice). El panel sigue siendo la cifra en vivo de hoy y no se toca.

### Reporte mensual (PDF)
- **Query:** `mes` (`AAAA-MM`, obligatorio): un mes cerrado o el actual, de los últimos 12 y no anterior al alta del negocio; si no, 422.
- **Contenido:** nombre del negocio, mes (el en curso dice «hasta el N»), totales del mes y del anterior con su cambio, serie por semana, top 3 cupones por canjes (título y canjes, de `cupones_metricas` en los días del mes), calificación y aspectos del mes (de `aspectos_suma` de los días, congelados como el resto) y la frase. Sin nombres, correos ni textos de clientes. Premium o no, mismo contenido básico; las avanzadas solo con premium.
- **Formato:** `application/pdf`, `Content-Disposition: attachment; filename="wherever-<mes>.pdf"`, una a dos páginas, solo texto y tablas (Helvetica, WinAnsi para acentos).
- **Generador:** un escritor mínimo propio en `internal/estadisticas/pdf` (PDF 1.4, texto y líneas, hoja carta, ~200 líneas, con pruebas que revisan la `xref` y leen el texto con `estadisticastest.Texto`). No hay librería de PDF en `go.mod` y la constitución §3 exige evaluar cada una; `go-pdf/fpdf` está archivada. Alternativa para el revisor: `github.com/signintech/gopdf` (MIT) con excepción de nv-00.
- Sin caché: el PDF se arma en < 100 ms con datos ya agregados; guardarlo en Redis gastaría cuota y memoria de Upstash.

### Paquete
- `internal/estadisticas`: modelo, repositorio, cálculo, reporte, trabajo (`Trabajo` cada hora como `reservas.Barrido`, `Tarea` nocturna y `Observador`), service, handler, DTO y `pdf`. Lee las otras colecciones con agregaciones de solo lectura en su repositorio, por nombre de colección, sin importar sus paquetes; `fuentes_test.go` comprueba que nombres y estados sigan iguales a los de cada módulo. `vistas` recibe el contador por la interfaz `vistas.Contador` (`WithContador`).

### Privacidad
- `estadisticas_diarias`, `estadisticas_categoria` y `estadisticas_relleno` no tienen `usuario_id`; no entran en ARCO (no son datos personales) y la exportación no cambia. Ningún log lleva ids de clientes.

### Costos
- **Redis:** cada `GET` cuesta lo de una lectura con sesión (política global 3 + revocación 1 ≈ 4). El trabajo y el contador de vistas no usan Redis. `TestPresupuestoRedis_EstadisticasNV74`.
- **MongoDB:** una escritura más por vista contada; el trabajo por hora toca solo negocios con actividad en hoy o ayer.
- **Gemini:** ninguno.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/negocios/{id}/estadisticas?desde=&hasta=&agrupar=` | JWT dueño (comerciante) | 200, 401, 403, 404, 422, 429 |
| GET | `/api/v1/negocios/{id}/reportes/mensual?mes=` | JWT dueño (comerciante) | 200 `application/pdf`, 401, 403, 404, 422, 429 |

Filas copiadas del Excel (A5): `GET /api/v1/negocios/{id}/estadisticas` «Visitas por día y hora, origen de las visitas y aspectos contra el promedio de su categoría, con la frase en lenguaje sencillo.» (Query: desde, hasta, agrupar (dia|semana); 200; Dueño (avanzadas solo premium)); `GET /api/v1/negocios/{id}/reportes/mensual?mes=` «Reporte mensual en PDF para el socio o contador (US-144).» (Query: mes (AAAA-MM); 200 application/pdf; Dueño). Filtro (A5 §7): «Estadísticas por rango» — rango de fechas y agrupación por día o semana; compara contra el periodo anterior; índice `ux_negocio_fecha`. Se copian a `docs/endpoints.md` al ejecutar.

**Drift con el Excel:** el Excel dice «un trabajo cada hora» y no menciona cómo contar vistas; aquí las vistas se cuentan en vivo y el resto cada hora. `ux_negocio_fecha` se queda con ese nombre. Se agrega `estadisticas_categoria` para no agregar todas las reseñas de la categoría en cada petición, y `estadisticas_relleno` para saber qué negocios ya tienen sus 90 días.

## Tests
- `tests/integration/estadisticas_test.go`: trabajo con reservas, check-ins, cupones, favoritos y reseñas da las cifras esperadas (la reserva apartada y el check-in inválido no cuentan); recalcular dos veces no duplica; borrar una cuenta no cambia un día cerrado; baja del negocio borra sus días; relleno de 90 días una sola vez; vista cuenta una vez por ventana de 10 min; agrupar por semana, periodo anterior, `cambio` y frase; sin premium `avanzadas: null`; categoría con 4 negocios = `categoria: null` y con 5 sí.
- `tests/integration/reporte_mensual_test.go`: PDF válido (cabecera `%PDF-`, `Content-Disposition`, se extrae el texto con el negocio, el nombre del mes y los totales), `mes` futuro, de hace 13 meses, inválido o ausente = 422.
- `tests/security/estadisticas_security_test.go`: 401 sin token, 403 consumidor, dueño ajeno = 404, el PDF y la respuesta no traen datos de clientes.
- `tests/integration/presupuesto_redis_test.go` (`TestPresupuestoRedis_EstadisticasNV74`): los dos `GET` = lectura con sesión; contador, trabajo por hora, relleno y nocturno con 0 comandos.
- `tests/database/indexes_test.go`: `ux_negocio_fecha` único y el de `estadisticas_categoria`. `tests/contract/estadisticas_contract_test.go`: 200 de estadísticas y cabeceras del PDF.
- Unit `internal/estadisticas`: rango y `agrupar`, semanas, porcentaje de cambio (anterior en 0 = `null`), frase por reglas, mes del reporte, aspectos sin categoría, nombres de las fuentes; `internal/estadisticas/pdf`: `xref`, acentos en WinAnsi, paréntesis escapados y salto de página.

## Criterios
- Las cifras de un día cerrado no cambian con borrados de cuentas y nunca llevan datos de personas.
- El negocio solo ve lo suyo; la categoría solo se compara con 5 negocios o más.
- Las estadísticas no gastan Redis más allá de una lectura ni Gemini.

## Límites
- Sin consejo con IA (nv-75), sin estado de cuenta (`estado-cuenta`, dinero de nv-60) ni facturas CFDI.
- Sin exportar a CSV ni enviar el reporte por correo.
- Sin estadísticas en tiempo real (el panel de nv-56 ya cubre hoy).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-16), ola 12; implementada en la rama `claude/laughing-khorana-71fa8b`, sin firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** vistas contadas en vivo en `estadisticas_diarias` (no desde `historial_vistas`); resto por trabajo cada hora solo con MongoDB; días cerrados congelados y anónimos; avanzadas premium devuelven `null` con 200; categoría solo con ≥ 5 negocios; frase por reglas sin IA; PDF con escritor propio sin dependencia nueva (o `gopdf` con excepción). **Aplicadas tal como se proponen (2026-11-16)**: escritor de PDF propio y `go.mod` sin cambios.
- **Detalle:** evidencia en `specs/nv-74/checklists/gates.md` + `Memoria.md` al ejecutar.
