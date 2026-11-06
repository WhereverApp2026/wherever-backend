# gates.md — NV-14 Infraestructura como paso configurado

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos; sin claves de AWS en GitHub | `critic-guardrails: PASS`; `gh secret list` vacío | listo |
| critic B2 | Sin credenciales en la salida de los scripts | Salida del ciclo real revisada: `deploy-env` dice "valores no mostrados" | listo |
| critic B3 | Pipeline sin cambios | `critic-guardrails: PASS` | listo |
| critic B4 | Endpoints sin cambios | `critic-guardrails: PASS` | listo |
| Solo plan | `--solo-plan` de ambos no cambia nada | `crear`: sin cambios; `destruir`: 41 recursos listados, ninguno borrado | listo |
| Confirmación | Sin la palabra exacta, no cambian nada | `destruir.sh` con otra palabra: cancelado. `crear.sh` y `destruir.sh` en el ciclo real leyeron su palabra exacta. No se volvió a probar `crear.sh` con otra palabra | listo |
| Servidor vivo | El instalador no repite el primer arranque ni reinicia la API | `ya tiene .env; sin primer arranque`; misma hora de inicio del contenedor antes y después | listo |
| Primer arranque | Una instancia nueva se instala sola | La B de nv-12: `crear.sh` la preparó; en cuanto Atlas permitió su IP, instaló y quedó sana | listo |
| Ciclo | `destruir.sh` y `crear.sh` recrean todo; `/ready` 200 | 2026-11-06: 55 recursos borrados y 53 creados, en 25 minutos con los pasos manuales; 2 instancias sanas; `/ready` 200 y `verificar: OK` en las dos | listo |
| Pasos manuales | Atlas, SNS y URL listados y suficientes | Bastaron los tres; una IP mal cargada en Atlas dejó a una instancia sin base hasta corregirla | listo |
