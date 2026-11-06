# Balanceador de las dos APIs (spec nv-12). Solo CloudFront llega a él; el HTTPS lo da CloudFront.

data "aws_ec2_managed_prefix_list" "cloudfront" {
  name = "com.amazonaws.global.cloudfront.origin-facing"
}

resource "aws_security_group" "alb" {
  name        = "${local.name}-alb"
  description = "Balanceador: solo recibe de CloudFront y solo habla con las APIs"
  vpc_id      = data.aws_vpc.default.id
}

# La lista de prefijos cuenta como unas 55 reglas de la cuota de 60 por grupo: no le agregues más.
resource "aws_vpc_security_group_ingress_rule" "alb_from_cloudfront" {
  security_group_id = aws_security_group.alb.id
  description       = "Solo desde CloudFront"
  prefix_list_id    = data.aws_ec2_managed_prefix_list.cloudfront.id
  ip_protocol       = "tcp"
  from_port         = 80
  to_port           = 80
}

resource "aws_vpc_security_group_egress_rule" "alb_to_api" {
  security_group_id            = aws_security_group.alb.id
  description                  = "Hacia el puerto de la API"
  referenced_security_group_id = aws_security_group.api.id
  ip_protocol                  = "tcp"
  from_port                    = var.app_port
  to_port                      = var.app_port
}

resource "aws_vpc_security_group_ingress_rule" "api_from_alb" {
  security_group_id            = aws_security_group.api.id
  description                  = "API solo desde el balanceador"
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = var.app_port
  to_port                      = var.app_port
}

# Público porque CloudFront entra por internet; el grupo de seguridad deja pasar solo a CloudFront.
#trivy:ignore:AWS-0053
resource "aws_lb" "api" {
  name                       = "${local.name}-alb"
  load_balancer_type         = "application"
  internal                   = false
  security_groups            = [aws_security_group.alb.id]
  subnets                    = [for subnet in data.aws_subnet.api : subnet.id]
  drop_invalid_header_fields = true
  # Más que el plazo de CloudFront (60 s) y menos que el de la API (120 s): evita 502 por conexión vieja.
  idle_timeout = 65
}

resource "aws_lb_target_group" "api" {
  name     = "${local.name}-api"
  port     = var.app_port
  protocol = "HTTP"
  vpc_id   = data.aws_vpc.default.id
  # El vaciado que espera actualizar.sh antes de reiniciar la API.
  deregistration_delay = 30

  # /health y no /ready: no gasta Redis y una pausa de Aura no saca a las dos instancias.
  health_check {
    path                = "/health"
    matcher             = "200"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 2
  }
}

resource "aws_lb_target_group_attachment" "api" {
  for_each         = aws_instance.api
  target_group_arn = aws_lb_target_group.api.arn
  target_id        = each.value.id
  port             = var.app_port
}

# HTTP porque el tramo es CloudFront → balanceador, dentro de AWS; aceptado en desarrollo sin dominio.
#trivy:ignore:AWS-0054
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.api.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}

# La propia instancia se saca y se vuelve a registrar al instalar una imagen.
data "aws_iam_policy_document" "manage_targets" {
  statement {
    sid       = "RegistrarYQuitarDestinos"
    actions   = ["elasticloadbalancing:RegisterTargets", "elasticloadbalancing:DeregisterTargets"]
    resources = [aws_lb_target_group.api.arn]
  }
  # AWS no permite acotar esta lectura a un recurso.
  statement {
    sid       = "LeerSaludDeDestinos"
    actions   = ["elasticloadbalancing:DescribeTargetHealth"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "manage_targets" {
  name   = "manejar-destinos"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.manage_targets.json
}

# --- Alarmas del balanceador (nv-13). Con las instancias apagadas no hay destinos no sanos. ---

locals {
  alb_alarms = {
    destinos-no-sanos = { metric = "UnHealthyHostCount", statistic = "Maximum", threshold = 1, description = "Una API no pasa el chequeo de salud" }
    balanceador-5xx   = { metric = "HTTPCode_ELB_5XX_Count", statistic = "Sum", threshold = 20, description = "20 o más 5xx del balanceador en 5 minutos" }
  }
}

resource "aws_cloudwatch_metric_alarm" "alb" {
  for_each            = local.alb_alarms
  alarm_name          = "${local.name}-${each.key}"
  alarm_description   = each.value.description
  namespace           = "AWS/ApplicationELB"
  metric_name         = each.value.metric
  statistic           = each.value.statistic
  period              = 300
  evaluation_periods  = 1
  threshold           = each.value.threshold
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  ok_actions          = [aws_sns_topic.alarms.arn]
  dimensions = merge(
    { LoadBalancer = aws_lb.api.arn_suffix },
    each.value.metric == "UnHealthyHostCount" ? { TargetGroup = aws_lb_target_group.api.arn_suffix } : {},
  )
}
