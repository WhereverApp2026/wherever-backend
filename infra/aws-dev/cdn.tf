# CloudFront da el HTTPS con su certificado de *.cloudfront.net, porque no hay dominio propio (spec nv-12).

data "aws_cloudfront_cache_policy" "disabled" {
  name = "Managed-CachingDisabled"
}

# Reenvía todo lo del cliente, incluido Authorization, salvo Host, que debe ser el del balanceador.
data "aws_cloudfront_origin_request_policy" "all_viewer" {
  name = "Managed-AllViewerExceptHostHeader"
}

# Sin WAF ni registros de acceso: entorno de desarrollo, y cuestan; los logs de la API ya van a CloudWatch.
#trivy:ignore:AWS-0010
#trivy:ignore:AWS-0011
resource "aws_cloudfront_distribution" "api" {
  enabled         = true
  comment         = "${local.name}-api"
  price_class     = "PriceClass_100"
  is_ipv6_enabled = true
  http_version    = "http2"

  origin {
    origin_id   = "balanceador"
    domain_name = aws_lb.api.dns_name

    custom_origin_config {
      http_port                = 80
      https_port               = 443
      origin_protocol_policy   = "http-only"
      origin_ssl_protocols     = ["TLSv1.2"]
      origin_read_timeout      = 60
      origin_keepalive_timeout = 5
    }
  }

  default_cache_behavior {
    target_origin_id         = "balanceador"
    viewer_protocol_policy   = "redirect-to-https"
    allowed_methods          = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods           = ["GET", "HEAD"]
    cache_policy_id          = data.aws_cloudfront_cache_policy.disabled.id
    origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer.id
    # La API comprime según Accept-Encoding; con esta política CloudFront no lo haría.
    compress = false
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
