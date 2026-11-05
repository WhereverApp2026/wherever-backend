# Checklist — nv-08 Despliegue continuo al entorno de desarrollo

> Trazabilidad: `spec R1-R15 ↔ plan §3-§5 ↔ task T1-T9`. Estado: Borrador con implementación local, sin desplegar.

- [ ] Solo publica un `push` a `main` con `ci-pr` en verde; PR, rama, fork o CI cancelado no publican (R1) — push a `main` publicó (runs 41988428645 y 42077644849); los PR #13 a #16 no dispararon `cd-dev`. CI cancelado sin ver
- [ ] Imagen del `Dockerfile` con el SHA del commit, visible en `/health` (R2) — publicada por el run 41988428645 (`7433d30`); `/health` ya muestra ese SHA
- [x] GitHub sin credenciales de AWS y sin forma de mandar comandos al servidor (R3) — `cd-dev.yml` no pide `id-token` y el servidor jala por su cuenta; el repositorio no tiene secretos
- [ ] GHCR privado con `GITHUB_TOKEN`; la EC2 baja con PAT `read:packages` que no llega a la API (R4) — publicar y bajar con el PAT probados en local; la imagen trae `/api` y `/indexes`, ligada al repo, sin secretos; en la EC2 comprobado: `GHCR_TOKEN` no está en `.env` ni en la API, y no queda configuración de Docker
- [ ] El timer revisa cada minuto y al arrancar; el arranque no registra fallos falsos (R5) — instalado el 2026-11-05: instaló `dev` en 11 s (`d2dc808` → `7433d30`) y las revisiones siguientes no reinstalan; falta ver un arranque de la instancia
- [ ] `/indexes` corre antes de cambiar la API y es idempotente (R6)
- [ ] Una versión rota regresa sola, queda en `failed_digest` y no se reintenta; el workflow termina en rojo (R7) — **servidor comprobado, ver abajo**; falta ver el rojo del workflow
- [ ] De 21:50 a 22:00 el servidor no actualiza y el workflow avisa en verde (R8)
- [ ] Un solo despliegue a la vez, en el workflow y en el servidor (R9)
- [ ] Regreso manual por SHA con re-etiquetado: con imagen, sin imagen y no ancestro (R10) — con imagen y no ancestro comprobados, ver abajo; sin imagen, sin ver
- [x] `sha` inválido rechazado antes de tocar GHCR; nunca interpolado en un comando (R11) — run 42081539887: `abc;touch /tmp/x` falla en `validar sha` y salta todo lo demás
- [x] Repetir el mismo SHA termina en verde sin construir (R12) — run 42081641837: "ya existe; no se reconstruye"
- [ ] `wherever-dev-boundary` en los dos roles; `plan` revisado, estado respaldado, ciclo de apagado comprobado (R13) — aplicado el 2026-11-04: 2 cambios, nada reemplazado, `plan` posterior sin cambios, SSM `Online`, `/ready` 200, `verificar.sh` OK; ciclo real visto: encendió sola el 2026-11-05 a las 08:00:47 de México con el boundary puesto, SSM `Online` y `/ready` 200
- [ ] GHCR con 10 versiones como máximo y la imagen activa intacta tras limpiar (R14) — **Hallazgo corregido:** cada publicación crea 2 versiones (imagen e índice de `dev`), así que 5 versiones eran unos 2 o 3 despliegues; ahora son 10. Falta ver la limpieza al pasar de 10
- [ ] Tabla de `/health` comprobada: SHA nuevo, sin conexión y SHA antiguo (R15)
- [ ] `actionlint`, `yaml.safe_load`, acciones fijadas por SHA, `critic`, `check_sdd`, `check_endpoints`, `terraform fmt`/`validate`
- [ ] Enmienda de la constitución §3 y §8, incluida la frase "`main` protegida"
- [ ] `wherever-sin-limites` retirado del servidor
- [ ] `Memoria.md` con el cierre de la excepción del binario local y el mantenimiento manual de los scripts
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia de CD real (2026-11-05)
- **Camino feliz:** fusión del PR #16 a las 17:39:05 UTC; `ci-pr` verde; `cd-dev` (run 42077644849) publicó; el servidor instaló a las 17:47:32 y `/health` mostró `7d5c534` a las 17:47:37; `cd-dev` terminó en verde con "el servidor ya sirve". API reiniciada en unos 4 s.
- **Regreso automático:** imagen `prueba-rota`, igual a la buena pero con `ENTRYPOINT ["/indexes"]`: la API termina y nunca responde; `/indexes` pasa. `dev` movida a las 18:06:31.
  - +27 s: instala; `indexes: OK`.
  - +33 s: `/health` da 502.
  - +156 s: `/health` 200 en `7d5c534`, `/ready` 200; digest en `failed_digest`. Caída de unos 2 min.
  - Sin reintentos: 0 instalaciones después del regreso, con `dev` aún en la imagen rota.
- **Regreso manual:** `dev` devuelta con `workflow_dispatch` (run 42081641837), sin reconstruir y sin reiniciar la API, porque coincide con el digest activo.
- **Fuera de `main`:** run 42081590126, SHA del PR #14: "no es ancestro de main", sin publicar.
- **Encendido con el boundary:** 2026-11-05 08:00:47 de México, SSM `Online`, `/ready` 200. La instancia lee parámetros y descifra `GHCR_TOKEN`.
- **Sin ver todavía:** arranque con una versión nueva esperando, ventana de 21:50 a 22:00, rojo del workflow con el SHA antiguo, reconstrucción de un SHA sin imagen, CI cancelado.
