# gates.md — NV-14 Infraestructura como paso configurado

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos; sin claves de AWS en GitHub | `critic-guardrails: PASS`; `gh secret list` vacío | listo |
| critic B2 | Sin credenciales en la salida de los scripts | Salida revisada en modo plan; falta el ciclo real | parcial |
| critic B3 | Pipeline sin cambios | `critic-guardrails: PASS` | listo |
| critic B4 | Endpoints sin cambios | `critic-guardrails: PASS` | listo |
| Solo plan | `--solo-plan` de ambos no cambia nada | `crear`: sin cambios; `destruir`: 41 recursos listados, ninguno borrado | listo |
| Confirmación | Sin la palabra exacta, no cambian nada | `destruir.sh` con otra palabra: cancelado. `crear.sh` no pidió confirmar porque no había cambios: falta verlo con un plan real | parcial |
| Servidor vivo | El instalador con `--site` no repite el primer arranque ni reinicia la API | `ya tiene .env; sin primer arranque`; misma hora de inicio del contenedor antes y después | listo |
| Ciclo | `destruir.sh` y `crear.sh` recrean todo; `/ready` 200 | salida | pendiente |
| Pasos manuales | Atlas, SNS y URL listados y suficientes | `README.md` y salida | pendiente |
