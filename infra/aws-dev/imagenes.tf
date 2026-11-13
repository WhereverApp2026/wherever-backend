# Imágenes (spec nv-44): bucket privado que solo CloudFront lee, por control de acceso de origen.
# Prefijos: `cuarentena/` (subidas sin moderar; nunca sale por CloudFront y se borra a las 24 h)
# e `i/` (publicadas, nombres inmutables). nv-45 sube, modera y publica.

locals {
  imagenes_bucket      = "${local.name}-imagenes-${data.aws_caller_identity.current.account_id}"
  imagenes_cuarentena  = "cuarentena/"
  imagenes_publicadas  = "i/"
  imagenes_cache_1_ano = 31536000
}

# Sin versiones ni registros de acceso: las imágenes se pueden volver a subir y cada objeto
# guardado cuesta; los accesos ya pasan por CloudFront. force_destroy: destruir.sh (nv-14) no
# puede borrar un bucket con objetos.
#trivy:ignore:AWS-0089
#trivy:ignore:AWS-0090
resource "aws_s3_bucket" "imagenes" {
  bucket        = local.imagenes_bucket
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "imagenes" {
  bucket                  = aws_s3_bucket.imagenes.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Sin ACL: el dueño del bucket es dueño de todo objeto.
resource "aws_s3_bucket_ownership_controls" "imagenes" {
  bucket = aws_s3_bucket.imagenes.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

# SSE-S3 y no KMS: sin costo por llamada y sin política de llave para CloudFront.
#trivy:ignore:AWS-0132
resource "aws_s3_bucket_server_side_encryption_configuration" "imagenes" {
  bucket = aws_s3_bucket.imagenes.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# S3 aplica la expiración en su ciclo diario: un objeto puede durar hasta unas 48 h.
resource "aws_s3_bucket_lifecycle_configuration" "imagenes" {
  bucket = aws_s3_bucket.imagenes.id

  rule {
    id     = "vaciar-cuarentena"
    status = "Enabled"
    filter {
      prefix = local.imagenes_cuarentena
    }
    expiration {
      days = 1
    }
  }

  rule {
    id     = "subidas-incompletas"
    status = "Enabled"
    filter {}
    abort_incomplete_multipart_upload {
      days_after_initiation = 1
    }
  }
}

data "aws_iam_policy_document" "imagenes_bucket" {
  statement {
    sid       = "SoloCloudFrontLeePublicadas"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.imagenes.arn}/${local.imagenes_publicadas}*"]
    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.imagenes.arn]
    }
  }

  statement {
    sid       = "SoloHTTPS"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.imagenes.arn, "${aws_s3_bucket.imagenes.arn}/*"]
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

resource "aws_s3_bucket_policy" "imagenes" {
  bucket = aws_s3_bucket.imagenes.id
  policy = data.aws_iam_policy_document.imagenes_bucket.json
  # El bloqueo de acceso público va primero: así S3 rechaza cualquier política pública.
  depends_on = [aws_s3_bucket_public_access_block.imagenes]
}

# --- Rol de la instancia: escribir en cuarentena, leer para moderar, publicar y borrar. Sin
# s3:ListBucket: la API conoce cada clave y un objeto inexistente da 403, no 404. ---

data "aws_iam_policy_document" "imagenes_instancia" {
  statement {
    sid     = "ImagenesDelProyecto"
    actions = ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"]
    resources = [
      "${aws_s3_bucket.imagenes.arn}/${local.imagenes_cuarentena}*",
      "${aws_s3_bucket.imagenes.arn}/${local.imagenes_publicadas}*",
    ]
  }
}

resource "aws_iam_role_policy" "imagenes" {
  name   = "imagenes"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.imagenes_instancia.json
}

# --- CloudFront propio para imágenes: HTTPS con el certificado de *.cloudfront.net. ---

resource "aws_cloudfront_origin_access_control" "imagenes" {
  name                              = "${local.name}-imagenes"
  description                       = "Lectura del bucket de imagenes"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# Un año en todas las capas: el nombre de cada imagen es aleatorio y no cambia.
resource "aws_cloudfront_cache_policy" "imagenes" {
  name        = "${local.name}-imagenes"
  comment     = "Imagenes inmutables, un ano"
  min_ttl     = local.imagenes_cache_1_ano
  default_ttl = local.imagenes_cache_1_ano
  max_ttl     = local.imagenes_cache_1_ano

  parameters_in_cache_key_and_forwarded_to_origin {
    enable_accept_encoding_gzip   = false
    enable_accept_encoding_brotli = false
    cookies_config {
      cookie_behavior = "none"
    }
    headers_config {
      header_behavior = "none"
    }
    query_strings_config {
      query_string_behavior = "none"
    }
  }
}

data "aws_cloudfront_response_headers_policy" "security_headers" {
  name = "Managed-SecurityHeadersPolicy"
}

# Sin WAF ni registros de acceso, como la de la API (cdn.tf).
#trivy:ignore:AWS-0010
#trivy:ignore:AWS-0011
resource "aws_cloudfront_distribution" "imagenes" {
  enabled         = true
  comment         = "${local.name}-imagenes"
  price_class     = "PriceClass_100"
  is_ipv6_enabled = true
  http_version    = "http2and3"

  origin {
    origin_id                = "imagenes"
    domain_name              = aws_s3_bucket.imagenes.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.imagenes.id
  }

  default_cache_behavior {
    target_origin_id           = "imagenes"
    viewer_protocol_policy     = "redirect-to-https"
    allowed_methods            = ["GET", "HEAD"]
    cached_methods             = ["GET", "HEAD"]
    cache_policy_id            = aws_cloudfront_cache_policy.imagenes.id
    response_headers_policy_id = data.aws_cloudfront_response_headers_policy.security_headers.id
    # JPEG ya va comprimido.
    compress = false
  }

  # Un 403 de S3 (clave inexistente o fuera de i/) no se guarda un año.
  custom_error_response {
    error_code            = 403
    error_caching_min_ttl = 60
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}
