data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

# Amazon Linux 2023 más reciente (x86_64 para t3; arm64 para t4g).
data "aws_ssm_parameter" "al2023" {
  name = startswith(var.instance_type, "t4g") ? "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64" : "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

data "aws_vpc" "default" {
  default = true
}

locals {
  name         = "${var.project}-${var.environment}"
  param_path   = "/${var.project}/${var.environment}/"
  site_address = var.domain_name != "" ? var.domain_name : "${replace(aws_eip.api["a"].public_ip, ".", "-")}.sslip.io"
  # Dos zonas distintas (nv-12). "a" es la instancia original: su zona no se puede cambiar sin reemplazarla.
  api_nodes = { a = "${var.region}c", b = "${var.region}a" }
  # Tope de permisos de todo rol del proyecto (nv-08). Lo crea el administrador en la consola.
  boundary_arn = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:policy/${local.name}-boundary"
}

# --- Red: el puerto de la API solo desde el balanceador (alb.tf); 22 cerrado; ninguna base. ---

resource "aws_security_group" "api" {
  name        = "${local.name}-api"
  description = "Wherever dev: HTTPS publico; SSH y API solo desde la IP de quien desarrolla"
  vpc_id      = data.aws_vpc.default.id
}

# Caddy y sslip.io siguen hasta que la URL de CloudFront pase las pruebas (nv-12).
moved {
  from = aws_vpc_security_group_ingress_rule.http
  to   = aws_vpc_security_group_ingress_rule.http[0]
}

moved {
  from = aws_vpc_security_group_ingress_rule.https
  to   = aws_vpc_security_group_ingress_rule.https[0]
}

resource "aws_vpc_security_group_ingress_rule" "http" {
  count             = var.caddy_enabled ? 1 : 0
  security_group_id = aws_security_group.api.id
  description       = "HTTP para el reto de Lets Encrypt y la redireccion a HTTPS"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 80
  to_port           = 80
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  count             = var.caddy_enabled ? 1 : 0
  security_group_id = aws_security_group.api.id
  description       = "HTTPS publico (Caddy)"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  count             = var.ssh_enabled ? 1 : 0
  security_group_id = aws_security_group.api.id
  description       = "SSH solo desde la IP de quien desarrolla"
  cidr_ipv4         = var.ssh_cidr
  ip_protocol       = "tcp"
  from_port         = 22
  to_port           = 22
}

# Salida solo por los puertos que hacen falta. El destino es cualquier IP porque las de Atlas,
# Let's Encrypt, los repositorios de paquetes y los registros de imágenes cambian; el DNS y el
# NTP de la VPC no pasan por el Security Group. Excepción documentada en Memoria.md.
locals {
  egress_ports = {
    https = { port = 443, why = "HTTPS: Lets Encrypt, imagenes, paquetes y AWS" }
    http  = { port = 80, why = "HTTP: repositorios de paquetes de Amazon Linux" }
    atlas = { port = 27017, why = "MongoDB Atlas (Cluster0)" }
    redis = { port = 6379, why = "Upstash Redis con TLS (nv-11)" }
    neo4j = { port = 7687, why = "Neo4j Aura con TLS (nv-11)" }
  }
}

#trivy:ignore:AWS-0104
resource "aws_vpc_security_group_egress_rule" "out" {
  for_each          = local.egress_ports
  security_group_id = aws_security_group.api.id
  description       = each.value.why
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = each.value.port
  to_port           = each.value.port
}

# --- Rol de la instancia: Systems Manager y solo los parámetros de /<proyecto>/<entorno>/. ---

data "aws_iam_policy_document" "ec2_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "instance" {
  name                 = "${local.name}-instance"
  assume_role_policy   = data.aws_iam_policy_document.ec2_assume.json
  permissions_boundary = local.boundary_arn
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

data "aws_iam_policy_document" "read_params" {
  statement {
    sid       = "LeerParametrosDelEntorno"
    actions   = ["ssm:GetParametersByPath", "ssm:GetParameters", "ssm:GetParameter"]
    resources = ["arn:aws:ssm:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:parameter${trimsuffix(local.param_path, "/")}", "arn:aws:ssm:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:parameter${local.param_path}*"]
  }
  statement {
    sid       = "DescifrarConLaLlaveAdministradaDeSSM"
    actions   = ["kms:Decrypt"]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "kms:ViaService"
      values   = ["ssm.${data.aws_region.current.region}.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "read_params" {
  name   = "leer-parametros"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.read_params.json
}

resource "aws_iam_instance_profile" "instance" {
  name = "${local.name}-instance"
  role = aws_iam_role.instance.name
}

# --- Instancia: tipo elegible, créditos standard, IMDSv2, disco gp3 cifrado. ---

resource "aws_key_pair" "dev" {
  key_name   = "${local.name}-ssh"
  public_key = var.ssh_public_key
}

data "aws_subnet" "api" {
  for_each          = local.api_nodes
  vpc_id            = data.aws_vpc.default.id
  availability_zone = each.value
  default_for_az    = true
}

moved {
  from = aws_instance.api
  to   = aws_instance.api["a"]
}

moved {
  from = aws_eip.api
  to   = aws_eip.api["a"]
}

moved {
  from = aws_eip_association.api
  to   = aws_eip_association.api["a"]
}

resource "aws_instance" "api" {
  for_each               = local.api_nodes
  subnet_id              = data.aws_subnet.api[each.key].id
  ami                    = data.aws_ssm_parameter.al2023.value
  instance_type          = var.instance_type
  key_name               = aws_key_pair.dev.key_name
  iam_instance_profile   = aws_iam_instance_profile.instance.name
  vpc_security_group_ids = [aws_security_group.api.id]
  monitoring             = false

  credit_specification {
    cpu_credits = "standard"
  }

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.volume_gb
    encrypted             = true
    delete_on_termination = true
  }

  user_data = templatefile("${path.module}/user_data.sh.tpl", {
    swap_gb    = var.swap_gb
    param_path = local.param_path
    region     = var.region
  })
  user_data_replace_on_change = false

  tags = {
    Name = "${local.name}-api-${each.key}"
  }

  lifecycle {
    ignore_changes = [ami]
  }
}

# IP fija: sin ella cada apagado cambiaría la IP, la URL de la app y el certificado.
resource "aws_eip" "api" {
  for_each = local.api_nodes
  domain   = "vpc"
  tags = {
    Name = "${local.name}-api-${each.key}"
  }
}

resource "aws_eip_association" "api" {
  for_each      = local.api_nodes
  instance_id   = aws_instance.api[each.key].id
  allocation_id = aws_eip.api[each.key].id
}

# --- Apagado diario y encendido entre semana (EventBridge Scheduler). ---

data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name                 = "${local.name}-scheduler"
  assume_role_policy   = data.aws_iam_policy_document.scheduler_assume.json
  permissions_boundary = local.boundary_arn
}

data "aws_iam_policy_document" "start_stop" {
  statement {
    sid       = "SoloEncenderYApagarEstaInstancia"
    actions   = ["ec2:StartInstances", "ec2:StopInstances"]
    resources = [for instance in aws_instance.api : instance.arn]
  }
}

resource "aws_iam_role_policy" "start_stop" {
  name   = "encender-apagar"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.start_stop.json
}

resource "aws_scheduler_schedule" "stop" {
  name                         = "${local.name}-apagar"
  description                  = "Apaga las instancias de desarrollo todos los días"
  schedule_expression          = var.stop_expression
  schedule_expression_timezone = var.timezone

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = "arn:aws:scheduler:::aws-sdk:ec2:stopInstances"
    role_arn = aws_iam_role.scheduler.arn
    input    = jsonencode({ InstanceIds = [for instance in aws_instance.api : instance.id] })
  }
}

resource "aws_scheduler_schedule" "start" {
  name                         = "${local.name}-encender"
  description                  = "Enciende las instancias de desarrollo de lunes a viernes"
  schedule_expression          = var.start_expression
  schedule_expression_timezone = var.timezone
  state                        = var.start_enabled ? "ENABLED" : "DISABLED"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = "arn:aws:scheduler:::aws-sdk:ec2:startInstances"
    role_arn = aws_iam_role.scheduler.arn
    input    = jsonencode({ InstanceIds = [for instance in aws_instance.api : instance.id] })
  }
}

# --- Presupuesto con alerta por correo. ---

resource "aws_budgets_budget" "monthly" {
  name         = "${local.name}-mensual"
  budget_type  = "COST"
  limit_amount = tostring(var.budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  # Sin descontar créditos: en el plan gratuito nuevo el gasto sale de los US$100 de crédito y,
  # contándolos, el presupuesto siempre marcaría cero. Así la alerta muestra lo que se consume.
  cost_types {
    include_credit = false
    include_refund = false
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 50
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.budget_email]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.budget_email]
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.budget_email]
  }
}
