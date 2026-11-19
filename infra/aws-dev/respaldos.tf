# Respaldo semanal de MongoDB Atlas (M0 no tiene respaldos). La instancia a corre
# scripts/respaldo-mongo.sh con wherever-respaldo.timer y sube `respaldos/AAAA-MM-DD.archive.gz`.
# La instancia solo puede escribir: no lee, no lista ni borra. Una instancia comprometida puede
# subir basura con el nombre de un respaldo, pero el versionado guarda el anterior.

locals {
  respaldos_bucket  = "${local.name}-respaldos-${data.aws_caller_identity.current.account_id}"
  respaldos_prefijo = "respaldos/"
  # Cuatro semanas: cuatro respaldos a la mano. Las versiones reemplazadas duran lo mismo.
  respaldos_dias = 28
}

# Sin registros de acceso: nadie más que la instancia y el operador entra, y cada registro cuesta.
# force_destroy: destruir.sh (nv-14) no puede borrar un bucket con objetos; baja el último
# respaldo antes de destruir (README, «Respaldos de MongoDB»).
#trivy:ignore:AWS-0089
resource "aws_s3_bucket" "respaldos" {
  bucket        = local.respaldos_bucket
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "respaldos" {
  bucket                  = aws_s3_bucket.respaldos.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Sin ACL: el dueño del bucket es dueño de todo objeto.
resource "aws_s3_bucket_ownership_controls" "respaldos" {
  bucket = aws_s3_bucket.respaldos.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

# SSE-S3 y no KMS: una llave propia cuesta US$1 al mes, más que todo el bucket, y sumaría
# permisos de kms:GenerateDataKey al rol de la instancia y al boundary.
#trivy:ignore:AWS-0132
resource "aws_s3_bucket_server_side_encryption_configuration" "respaldos" {
  bucket = aws_s3_bucket.respaldos.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Versionado: si alguien sobrescribe un respaldo, la versión anterior sigue ahí 28 días.
resource "aws_s3_bucket_versioning" "respaldos" {
  bucket = aws_s3_bucket.respaldos.id
  versioning_configuration {
    status = "Enabled"
  }
}

# S3 aplica la expiración en su ciclo diario: un respaldo puede durar hasta un día más.
resource "aws_s3_bucket_lifecycle_configuration" "respaldos" {
  bucket = aws_s3_bucket.respaldos.id

  rule {
    id     = "cuatro-semanas"
    status = "Enabled"
    filter {}
    expiration {
      days = local.respaldos_dias
    }
    noncurrent_version_expiration {
      noncurrent_days = local.respaldos_dias
    }
    abort_incomplete_multipart_upload {
      days_after_initiation = 1
    }
  }

  # Tras expirar, solo queda la marca de borrado; esta regla la quita.
  rule {
    id     = "marcas-de-borrado"
    status = "Enabled"
    filter {}
    expiration {
      expired_object_delete_marker = true
    }
  }

  depends_on = [aws_s3_bucket_versioning.respaldos]
}

data "aws_iam_policy_document" "respaldos_bucket" {
  statement {
    sid       = "SoloHTTPS"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.respaldos.arn, "${aws_s3_bucket.respaldos.arn}/*"]
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
}

resource "aws_s3_bucket_policy" "respaldos" {
  bucket = aws_s3_bucket.respaldos.id
  policy = data.aws_iam_policy_document.respaldos_bucket.json
  # El bloqueo de acceso público va primero: así S3 rechaza cualquier política pública.
  depends_on = [aws_s3_bucket_public_access_block.respaldos]
}

# --- Rol de la instancia: solo escribir respaldos. `aws s3 cp` por partes (más de 8 MB) también
# entra en s3:PutObject. El boundary repite este techo (RespaldarMongo). ---

data "aws_iam_policy_document" "respaldos_instancia" {
  statement {
    sid       = "RespaldarMongo"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.respaldos.arn}/${local.respaldos_prefijo}*"]
  }
}

resource "aws_iam_role_policy" "respaldos" {
  name   = "respaldar-mongo"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.respaldos_instancia.json
}
