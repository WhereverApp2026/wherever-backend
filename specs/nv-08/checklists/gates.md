# Checklist — nv-08 Despliegue continuo al entorno de desarrollo

> Trazabilidad: `spec R1-R15 ↔ plan §3-§5 ↔ task T1-T9`. Estado: Borrador con implementación local, sin desplegar.

- [ ] Solo publica un `push` a `main` con `ci-pr` en verde; PR, rama, fork o CI cancelado no publican (R1)
- [ ] Imagen del `Dockerfile` con el SHA del commit, visible en `/health` (R2) — publicada por el run 41988428645 (`7433d30`); falta verla en `/health`
- [ ] GitHub sin credenciales de AWS y sin forma de mandar comandos al servidor (R3) — **evidencia en el reporte**
- [ ] GHCR privado con `GITHUB_TOKEN`; la EC2 baja con PAT `read:packages` que no llega a la API (R4) — publicar y bajar con el PAT probados en local; la imagen trae `/api` y `/indexes`, ligada al repo, sin secretos; falta en la EC2
- [ ] El timer revisa cada minuto y al arrancar; el arranque no registra fallos falsos (R5)
- [ ] `/indexes` corre antes de cambiar la API y es idempotente (R6)
- [ ] Una versión rota regresa sola, queda en `failed_digest` y no se reintenta; el workflow termina en rojo (R7) — **evidencia en el reporte**
- [ ] De 21:50 a 22:00 el servidor no actualiza y el workflow avisa en verde (R8)
- [ ] Un solo despliegue a la vez, en el workflow y en el servidor (R9)
- [ ] Regreso manual por SHA con re-etiquetado: con imagen, sin imagen y no ancestro (R10)
- [ ] `sha` inválido rechazado antes de tocar GHCR; nunca interpolado en un comando (R11)
- [ ] Repetir el mismo SHA termina en verde sin construir (R12)
- [ ] `wherever-dev-boundary` en los dos roles; `plan` revisado, estado respaldado, ciclo de apagado comprobado (R13) — aplicado el 2026-11-04: 2 cambios, nada reemplazado, `plan` posterior sin cambios, SSM `Online`, `/ready` 200, `verificar.sh` OK; ciclo real visto: encendió sola el 2026-11-05 a las 08:00:47 de México con el boundary puesto, SSM `Online` y `/ready` 200
- [ ] GHCR con 5 versiones como máximo y la imagen activa intacta tras limpiar (R14)
- [ ] Tabla de `/health` comprobada: SHA nuevo, sin conexión y SHA antiguo (R15)
- [ ] `actionlint`, `yaml.safe_load`, acciones fijadas por SHA, `critic`, `check_sdd`, `check_endpoints`, `terraform fmt`/`validate`
- [ ] Enmienda de la constitución §3 y §8, incluida la frase "`main` protegida"
- [ ] `wherever-sin-limites` retirado del servidor
- [ ] `Memoria.md` con el cierre de la excepción del binario local y el mantenimiento manual de los scripts
- [ ] Aprobación firmada: (pendiente de revisor)
