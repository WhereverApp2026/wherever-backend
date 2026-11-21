# Auditoría de la cuenta (plan de seguridad, control 18 de docs/seguridad.md): CloudTrail guarda
# quién hizo qué en AWS y avisa de cambios de IAM y del uso de root. Las alarmas de ataques a la
# API (picos de 401, 423 y 429) están en observabilidad.tf, sobre los logs de la API.
# Costo: la primera copia de los eventos de administración es gratis; S3 y CloudWatch Logs con
# este volumen son centavos; las dos métricas de aquí pasan las 10 gratis (≈US$0,60 al mes).

locals {
  auditoria_bucket    = "${local.name}-auditoria-${data.aws_caller_identity.current.account_id}"
  auditoria_log_group = "/${var.project}/${var.environment}/cloudtrail"
  trail_name          = "${local.name}-auditoria"
  trail_arn           = "arn:aws:cloudtrail:${var.region}:${data.aws_caller_identity.current.account_id}:trail/${local.trail_name}"

  # Filtros sobre los eventos de CloudTrail. `readOnly IS FALSE` deja fuera las lecturas.
  auditoria_filtros = {
    cambios-iam = {
      metric      = "CambiosIAM"
      pattern     = "{ ($.eventSource = \"iam.amazonaws.com\") && ($.readOnly IS FALSE) }"
      description = "Alguien cambió usuarios, roles o políticas de IAM"
    }
    uso-root = {
      metric      = "UsoDeRoot"
      pattern     = "{ ($.userIdentity.type = \"Root\") && ($.userIdentity.invokedBy NOT EXISTS) && ($.eventType != \"AwsServiceEvent\") }"
      description = "Se usó la cuenta root de AWS"
    }
  }
}

# --- Bucket de CloudTrail: privado, versionado y con los archivos de resumen que prueban que
# nadie editó un registro (validación de archivos de log). ---

# Sin registros de acceso: sería un bucket más para auditar este bucket, y cada registro cuesta.
# force_destroy: destruir.sh no puede borrar un bucket con objetos; la auditoría muere con el
# entorno, que es de desarrollo.
#trivy:ignore:AWS-0089
resource "aws_s3_bucket" "auditoria" {
  bucket        = local.auditoria_bucket
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "auditoria" {
  bucket                  = aws_s3_bucket.auditoria.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "auditoria" {
  bucket = aws_s3_bucket.auditoria.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

# SSE-S3 y no KMS: una llave propia cuesta US$1 al mes, más que toda la auditoría.
#trivy:ignore:AWS-0132
resource "aws_s3_bucket_server_side_encryption_configuration" "auditoria" {
  bucket = aws_s3_bucket.auditoria.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_versioning" "auditoria" {
  bucket = aws_s3_bucket.auditoria.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "auditoria" {
  bucket = aws_s3_bucket.auditoria.id

  rule {
    id     = "un-anio"
    status = "Enabled"
    filter {}
    expiration {
      days = var.auditoria_dias
    }
    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }

  rule {
    id     = "marcas-de-borrado"
    status = "Enabled"
    filter {}
    expiration {
      expired_object_delete_marker = true
    }
  }

  depends_on = [aws_s3_bucket_versioning.auditoria]
}

# CloudTrail solo escribe en AWSLogs/<cuenta>/ y solo para este trail (aws:SourceArn evita que
# otro trail de otra cuenta use el bucket).
data "aws_iam_policy_document" "auditoria_bucket" {
  statement {
    sid       = "SoloHTTPS"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.auditoria.arn, "${aws_s3_bucket.auditoria.arn}/*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
  statement {
    sid       = "CloudTrailLeeElPermiso"
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.auditoria.arn]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceArn"
      values   = [local.trail_arn]
    }
  }
  statement {
    sid       = "CloudTrailEscribe"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.auditoria.arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceArn"
      values   = [local.trail_arn]
    }
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }
}

resource "aws_s3_bucket_policy" "auditoria" {
  bucket     = aws_s3_bucket.auditoria.id
  policy     = data.aws_iam_policy_document.auditoria_bucket.json
  depends_on = [aws_s3_bucket_public_access_block.auditoria]
}

# --- CloudTrail también manda los eventos a CloudWatch Logs para filtrar y alarmar. ---

# Sin llave propia, como los logs de la API: cuesta US$1 al mes y los eventos ya están en S3.
#trivy:ignore:AWS-0017
resource "aws_cloudwatch_log_group" "auditoria" {
  name              = local.auditoria_log_group
  retention_in_days = var.log_retention_days
}

data "aws_iam_policy_document" "cloudtrail_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceArn"
      values   = [local.trail_arn]
    }
  }
}

resource "aws_iam_role" "cloudtrail" {
  name                 = "${local.name}-cloudtrail"
  assume_role_policy   = data.aws_iam_policy_document.cloudtrail_assume.json
  permissions_boundary = local.boundary_arn
}

# El boundary repite este techo (EnviarEventosDeAuditoria).
data "aws_iam_policy_document" "cloudtrail_logs" {
  statement {
    sid       = "EnviarEventosDeAuditoria"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.auditoria.arn}:*"]
  }
}

resource "aws_iam_role_policy" "cloudtrail" {
  name   = "enviar-eventos"
  role   = aws_iam_role.cloudtrail.id
  policy = data.aws_iam_policy_document.cloudtrail_logs.json
}

# Todas las regiones (alguien podría crear recursos en otra) y eventos globales (IAM vive en
# us-east-1). Solo administración: los eventos de datos (cada GetObject) se cobran.
# Sin llave KMS propia: los archivos ya van cifrados con SSE-S3 y la validación detecta cambios.
#trivy:ignore:AWS-0015
resource "aws_cloudtrail" "auditoria" {
  name                          = local.trail_name
  s3_bucket_name                = aws_s3_bucket.auditoria.id
  is_multi_region_trail         = true
  include_global_service_events = true
  enable_log_file_validation    = true
  cloud_watch_logs_group_arn    = "${aws_cloudwatch_log_group.auditoria.arn}:*"
  cloud_watch_logs_role_arn     = aws_iam_role.cloudtrail.arn

  depends_on = [aws_s3_bucket_policy.auditoria, aws_iam_role_policy.cloudtrail]
}

resource "aws_cloudwatch_log_metric_filter" "auditoria" {
  for_each       = local.auditoria_filtros
  name           = "${local.name}-${each.key}"
  log_group_name = aws_cloudwatch_log_group.auditoria.name
  pattern        = each.value.pattern

  metric_transformation {
    name      = each.value.metric
    namespace = local.metric_namespace
    value     = "1"
  }
}

# Un solo evento basta: en desarrollo nadie debería tocar IAM ni usar root sin saberlo el equipo.
resource "aws_cloudwatch_metric_alarm" "auditoria" {
  for_each            = local.auditoria_filtros
  alarm_name          = "${local.name}-${each.key}"
  alarm_description   = each.value.description
  namespace           = local.metric_namespace
  metric_name         = each.value.metric
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  depends_on = [aws_cloudwatch_log_metric_filter.auditoria]
}
