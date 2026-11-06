# Observabilidad — vivo

> Spec nv-13. Terraform: `infra/aws-dev/observabilidad.tf`. Sin datos sensibles: los oculta `internal/platform/logger`.

## Dónde está cada cosa
| Qué | Dónde |
|---|---|
| Logs de la API | CloudWatch, grupo `/wherever/dev/api`, flujo `api-<instancia>`, 14 días |
| Eventos de despliegue | Mismo grupo, flujo `despliegues-<instancia>` |
| Panel | CloudWatch → Paneles → `wherever-dev-api` |
| Consultas | Logs Insights → Consultas guardadas → `wherever-dev/*` |

```bash
MSYS_NO_PATHCONV=1 aws logs tail /wherever/dev/api --since 15m --profile wherever-dev --region us-east-2
```

## Campos que usan los filtros
Renombrar uno de estos campos en la API apaga su métrica en silencio.

| Campo | Lo escribe | Métrica |
|---|---|---|
| `level` | `slog` (`ERROR`, en mayúsculas) | `Errores` |
| `estado` | Línea de acceso | `Respuestas5xx`, `Peticiones` |
| `duracion_ms` | Línea de acceso | `LatenciaMs` |
| `dependencia` | `/ready` al encontrar una base caída | `DependenciaCaida` |
| `evento` | `actualizar.sh` | `Regresos` |

## Qué hacer con cada alarma
Sin datos no hay alarma: las instancias se apagan a diario.

| Alarma | Significa | Primer paso |
|---|---|---|
| `wherever-dev-errores` | 5 o más errores en 5 minutos | Consulta `errores-recientes` |
| `wherever-dev-respuestas` | 5 o más respuestas 5xx en 5 minutos | Consulta `errores-recientes`; un 503 suele ser Redis |
| `wherever-dev-dependencia` | `/ready` encontró una base caída | Busca `dependencia` en el log; revisa la pausa de Aura y la cuota de Upstash |
| `wherever-dev-regresos` | Una imagen nueva no quedó sana | Flujo `despliegues-*` y `/var/log/wherever/failed_digest` |
