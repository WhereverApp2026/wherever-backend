# gates.md — NV-13 Observabilidad en la nube

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | salida de `critic` | pendiente |
| critic B2 | Sin datos sensibles en los logs | `TestLogger_*`; falta revisar los logs reales | listo en local |
| critic B3 | Pipeline sin cambios | salida de `critic` | pendiente |
| critic B4 | Endpoints sin cambios | salida de `critic` | pendiente |
| Plan | Solo agrega recursos y cambia el rol de la instancia | 18 agregados, 0 cambios ni destrucciones; segundo `plan` sin cambios | listo |
| Logs | Petición de prueba visible en CloudWatch con los campos esperados | captura o salida | pendiente |
| Alarma | Error provocado dispara alarma y correo; base caída dispara la de `/ready` | captura | pendiente |
| Eventos | Despliegue visible en `despliegues-<instancia>` | salida | pendiente |
| Insights | Consultas guardadas devuelven resultados | captura | pendiente |
| Volumen | Consumo diario bajo la cuota gratuita | `Memoria.md` | pendiente |
| `verificar.sh` | exit 0 con controles nuevos | salida | pendiente |
