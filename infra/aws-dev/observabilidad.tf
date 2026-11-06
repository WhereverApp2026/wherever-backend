# Observabilidad en CloudWatch (spec nv-13): los logs de la API, sus métricas, alarmas y panel.
locals {
  log_group        = "/${var.project}/${var.environment}/api"
  metric_namespace = "Wherever/${title(var.environment)}"

  # Cada filtro crea una métrica; el plan gratuito cubre 10. Los campos los escribe el logger de la API.
  metric_filters = {
    errores     = { metric = "Errores", pattern = "{ $.level = \"ERROR\" }", value = "1" }
    respuestas  = { metric = "Respuestas5xx", pattern = "{ $.estado >= 500 }", value = "1" }
    peticiones  = { metric = "Peticiones", pattern = "{ $.estado > 0 }", value = "1" }
    latencia    = { metric = "LatenciaMs", pattern = "{ $.duracion_ms >= 0 }", value = "$.duracion_ms" }
    dependencia = { metric = "DependenciaCaida", pattern = "{ $.dependencia = * }", value = "1" }
    regresos    = { metric = "Regresos", pattern = "{ $.evento = \"regreso\" }", value = "1" }
  }

  # Con todo apagado no hay datos: "notBreaching" evita una alarma cada noche.
  alarms = {
    errores     = { metric = "Errores", threshold = 5, description = "5 o más errores en 5 minutos" }
    respuestas  = { metric = "Respuestas5xx", threshold = 5, description = "5 o más respuestas 5xx en 5 minutos" }
    dependencia = { metric = "DependenciaCaida", threshold = 1, description = "/ready encontró una base caída: MongoDB, Redis o Neo4j" }
    regresos    = { metric = "Regresos", threshold = 1, description = "Una imagen nueva no quedó sana y el servidor regresó a la anterior" }
  }

  # Las líneas de acceso y los eventos de despliegue son JSON; Insights lee sus campos directo.
  saved_queries = {
    errores-recientes = "fields @timestamp, msg, error, request_id | filter level = \"ERROR\" | sort @timestamp desc | limit 50"
    peticiones-lentas = "fields @timestamp, metodo, ruta, estado, duracion_ms | filter duracion_ms >= 1000 | sort duracion_ms desc | limit 50"
    login-fallido     = "stats count(*) as intentos by ruta, estado | filter ruta like \"/auth/login\" and estado >= 400 | sort intentos desc"
  }
}

# Sin llave propia: son logs de desarrollo sin datos sensibles (los filtra la API), y una llave cuesta.
#trivy:ignore:AWS-0017
resource "aws_cloudwatch_log_group" "api" {
  name              = local.log_group
  retention_in_days = var.log_retention_days
}

resource "aws_cloudwatch_log_metric_filter" "api" {
  for_each       = local.metric_filters
  name           = "${local.name}-${each.key}"
  log_group_name = aws_cloudwatch_log_group.api.name
  pattern        = each.value.pattern

  metric_transformation {
    name      = each.value.metric
    namespace = local.metric_namespace
    value     = each.value.value
  }
}

# Sin cifrado propio: CloudWatch no puede publicar en un tema cifrado con la llave administrada de SNS.
#trivy:ignore:AWS-0095
resource "aws_sns_topic" "alarms" {
  name = "${local.name}-alarmas"
}

# Queda pendiente hasta que el operador confirme el correo que manda AWS.
resource "aws_sns_topic_subscription" "alarms_email" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = var.budget_email
}

resource "aws_cloudwatch_metric_alarm" "api" {
  for_each            = local.alarms
  alarm_name          = "${local.name}-${each.key}"
  alarm_description   = each.value.description
  namespace           = local.metric_namespace
  metric_name         = each.value.metric
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = each.value.threshold
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  ok_actions          = [aws_sns_topic.alarms.arn]

  depends_on = [aws_cloudwatch_log_metric_filter.api]
}

resource "aws_cloudwatch_query_definition" "api" {
  for_each        = local.saved_queries
  name            = "${local.name}/${each.key}"
  log_group_names = [aws_cloudwatch_log_group.api.name]
  query_string    = each.value
}

resource "aws_cloudwatch_dashboard" "api" {
  dashboard_name = "${local.name}-api"
  dashboard_body = jsonencode({
    widgets = [
      for index, widget in [
        { title = "Errores y 5xx", metrics = ["Errores", "Respuestas5xx", "DependenciaCaida"], stat = "Sum" },
        { title = "Tráfico", metrics = ["Peticiones"], stat = "Sum" },
        { title = "Latencia (ms)", metrics = ["LatenciaMs"], stat = "p95" },
        ] : {
        type   = "metric"
        x      = (index % 2) * 12
        y      = floor(index / 2) * 6
        width  = 12
        height = 6
        properties = {
          title   = widget.title
          region  = var.region
          view    = "timeSeries"
          stat    = widget.stat
          period  = 300
          metrics = [for metric in widget.metrics : [local.metric_namespace, metric]]
        }
      }
    ]
  })
}

# El daemon de Docker envía los logs con el rol de la instancia; actualizar.sh, sus eventos.
data "aws_iam_policy_document" "send_logs" {
  statement {
    sid       = "EnviarLogsDeLaApi"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.api.arn}:*"]
  }
}

resource "aws_iam_role_policy" "send_logs" {
  name   = "enviar-logs"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.send_logs.json
}
