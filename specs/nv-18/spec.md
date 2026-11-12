# NV-18 — Caché en Redis

## Contexto
Una caché común reduce consultas a Mongo sin gastar de más la cuota gratuita de Redis. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-17.

## Requisitos
1. **Caché en Redis** en `internal/platform/cache` con un ayudante `cache.Obtener(ctx, c, clave, ttl, fn)` que evita avalanchas: las peticiones simultáneas de una clave en una instancia comparten una sola consulta al origen (`singleflight`). Con dos instancias, como máximo dos.
2. Se cachean las categorías (`GET /categorias`) y las fichas de negocio (`GET /lugares/{id}`) de `internal/negocios`. La importación de `POST /admin/negocios/importar` borra la caché de categorías y la ficha de cada negocio tocado.
3. La compresión ya está en producción (nv-12) y no se toca.

## Diseño
- **Llaves** (constitución §6): `cache:categorias:todas` y `cache:negocio:{id}`, siempre con TTL. Se guarda JSON.
- **TTL:** categorías 1 h, fichas 5 min. «Abierto ahora» no se guarda: la ficha guarda las ventanas de horario y se recalcula en cada petición.
- **Presupuesto de Redis:** acierto = 1 `GET`; fallo = `GET` + `SET`; invalidación = un solo `DEL` por importación. Máximo 2 comandos por petición cacheada, además del limitador.
- **Redis caído o cuota agotada:** cualquier error de Redis apaga la caché 30 s (`cache.Pausa`): no gasta comandos ni espera tiempos de espera, responde MongoDB y registra un solo aviso por pausa (`dependencia=redis`).
- **Importar:** `Repository.Upsert` usa `FindOneAndUpdate` para conocer el `_id` real al actualizar y borrar su ficha.
- **Cabeceras:** `GET /categorias` sigue con `no-store`; la caché es solo del servidor.
- **Fuera de alcance:** la configuración de nv-16 ya vive en memoria (se lee al arrancar), así que cachearla en Redis solo gastaría comandos. Las listas, el mapa y el autocompletar dependen de la consulta y multiplicarían llaves y comandos; no se cachean.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Caché 6` con Redis real (`tests/integration/cache_test.go`): acierto y fallo, «abierto ahora» recalculado, avalancha (50 peticiones, una consulta al origen), invalidación al importar (ficha y categorías), Redis caído (sigue respondiendo desde el origen).
- `TestPresupuestoRedis_CacheDeFicha`: el fallo suma 2 comandos y el acierto 1 sobre el limitador.
- Unit `internal/platform/cache`: caché nil, Redis caído, pausa, error del origen no guardado y avalancha.

## Criterios
- El consumo de Redis por caché queda medido y la estimación de nv-11 actualizada en `Memoria.md`.

## Límites
- La caché de Upstash tiene cuota mensual; si se agota, Redis da error y la caché se apaga sola antes de que falle la API.

## Aprobación y Control de Cambios
- **Estado:** ✅ Ejecutada (2026-11-12) en la rama `claude/nice-almeida-3fcaaa`; local verde (build, vet, Unit, lint, critic, sdd, endpoints). Integration, Security, Database y Contract en el CI del PR. Ola 4.
- **Revisores:** Osmar Reséndiz (pendiente de firmar)
- **Fecha:** 2026-11-12
- **Corrección de drift:** la spec original pedía cachear la configuración (ya en memoria) y no nombraba paquete ni llaves; ver Diseño.
- **Detalle:** evidencia en `specs/nv-18/checklists/gates.md` + `Memoria.md`.
