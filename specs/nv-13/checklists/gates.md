# gates.md — NV-13 Observabilidad en la nube

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` | listo |
| critic B2 | Sin datos sensibles en los logs | `TestLogger_*`; 31 líneas reales: 0 correos completos, 0 tokens, 0 credenciales | listo |
| critic B3 | Pipeline sin cambios | `critic-guardrails: PASS` | listo |
| critic B4 | Endpoints sin cambios | `check_endpoints: OK (37 rutas)` | listo |
| Plan | Solo agrega recursos y cambia el rol de la instancia | 18 agregados, 0 cambios ni destrucciones; segundo `plan` sin cambios | listo |
| Logs | Petición de prueba visible en CloudWatch con los campos esperados | `aws logs tail`: flujo público en menos de 15 s | listo |
| Alarma | Error provocado dispara alarma y correo; base caída dispara la de `/ready` | 7 errores y Neo4j caído en un contenedor aparte: `errores`, `respuestas` y `dependencia` en ALARM a los 50 s. **Falta el correo: suscripción sin confirmar** | parcial |
| Eventos | Despliegue visible en `despliegues-<instancia>` | La instalación de `61b5794` no dejó evento: la CLI rechazaba el JSON. Corregido y probado en el servidor con la función real de `actualizar.sh` | listo |
| Insights | Consultas guardadas devuelven resultados | Consulta de errores recientes: `Complete`, 7 registros | listo |
| Volumen | Consumo diario bajo la cuota gratuita | Estimado: unos 14 MB al mes frente a 5 GB; falta el dato real | estimado |
| `verificar.sh` | exit 0 con controles nuevos | `verificar: OK` | listo |
