# WAF en la distribución de la API (plan de seguridad, PR E; docs/seguridad.md §3.1). Filtra en el
# borde, antes de CloudFront → balanceador → API. Costo: US$5 el web ACL + US$1 por regla (4) +
# US$0.60 por millón de peticiones ≈ US$9–10 al mes. Los grupos administrados de AWS no cobran aparte.

# Un web ACL de CloudFront vive siempre en us-east-1, aunque el resto esté en us-east-2.
provider "aws" {
  alias   = "global"
  region  = "us-east-1"
  profile = var.profile

  default_tags {
    tags = {
      Proyecto = var.project
      Entorno  = var.environment
      Spec     = "nv-07"
      Gestion  = "terraform"
    }
  }
}

locals {
  origin_header = "X-Origen-Wherever"
  # Cuerpo uniforme de error de la API (constitución §4), para que la app móvil no distinga quién respondió.
  waf_bodies = {
    prohibido             = jsonencode({ codigo = "prohibido", mensaje = "sin permiso para este recurso", campos = [], traza_id = "" })
    demasiadas_peticiones = jsonencode({ codigo = "demasiadas_peticiones", mensaje = "demasiadas peticiones, intenta más tarde", campos = [], traza_id = "" })
  }
}

# Sin registro de peticiones: el grupo aws-waf-logs-* en us-east-1 cuesta por GB. Las métricas por
# regla y las muestras de peticiones bloqueadas (consola de WAF, últimas 3 h) bastan en desarrollo.
resource "aws_wafv2_web_acl" "api" {
  provider    = aws.global
  name        = "${local.name}-api"
  description = "Borde de la API: reputacion de IP, limite por IP y reglas administradas de AWS"
  scope       = "CLOUDFRONT"

  default_action {
    allow {}
  }

  dynamic "custom_response_body" {
    for_each = local.waf_bodies
    content {
      key          = custom_response_body.key
      content      = custom_response_body.value
      content_type = "APPLICATION_JSON"
    }
  }

  # IP con mala reputación según AWS (bots, escáneres, redes de ataque conocidas). Primero: es la más barata.
  rule {
    name     = "reputacion-ip"
    priority = 0

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        vendor_name = "AWS"
        name        = "AWSManagedRulesAmazonIpReputationList"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${local.name}-reputacion-ip"
      sampled_requests_enabled   = true
    }
  }

  # Corta inundaciones en el borde. Más alto que el límite global de la API (1000 por minuto por IP,
  # docs/rate-limit-matrix.md): una red escolar comparte IP y la API ya limita fino por ruta y usuario.
  rule {
    name     = "limite-por-ip"
    priority = 1

    action {
      block {
        custom_response {
          response_code            = 429
          custom_response_body_key = "demasiadas_peticiones"
          response_header {
            name  = "Retry-After"
            value = "300"
          }
        }
      }
    }

    statement {
      rate_based_statement {
        limit                 = var.waf_rate_limit
        evaluation_window_sec = 300
        aggregate_key_type    = "IP"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${local.name}-limite-por-ip"
      sampled_requests_enabled   = true
    }
  }

  # Entradas conocidas como maliciosas (Log4Shell, rutas de explotación de Java, host localhost).
  rule {
    name     = "entradas-maliciosas"
    priority = 2

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        vendor_name = "AWS"
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${local.name}-entradas-maliciosas"
      sampled_requests_enabled   = true
    }
  }

  # OWASP general (XSS, LFI, SSRF a metadatos, etc.). No mira subidas multipart: el binario de una foto
  # o un PDF da falsos positivos, y la API ya valida tipo, tamaño y contenido (nv-45, nv-55).
  # SizeRestrictions_BODY solo cuenta: bloquea cuerpos de más de 8 KB y la API acepta hasta 10 MB.
  rule {
    name     = "owasp-general"
    priority = 3

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        vendor_name = "AWS"
        name        = "AWSManagedRulesCommonRuleSet"

        rule_action_override {
          name = "SizeRestrictions_BODY"
          action_to_use {
            count {}
          }
        }

        scope_down_statement {
          not_statement {
            statement {
              byte_match_statement {
                search_string         = "multipart/form-data"
                positional_constraint = "STARTS_WITH"

                field_to_match {
                  single_header {
                    name = "content-type"
                  }
                }

                text_transformation {
                  priority = 0
                  type     = "LOWERCASE"
                }
              }
            }
          }
        }
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${local.name}-owasp-general"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${local.name}-api"
    sampled_requests_enabled   = true
  }
}

# --- Origen solo por esta distribución ---
# La lista de prefijos del balanceador deja entrar a cualquier CloudFront, también a una distribución
# ajena que apunte a nuestro balanceador y se salte el WAF. CloudFront agrega esta cabecera secreta y el
# balanceador solo reenvía si viene. Vive en el estado (local, fuera de Git). Rotarla:
#   terraform apply -replace=random_password.origen
resource "random_password" "origen" {
  length = 48
  # Sin * ni ?: la condición de cabecera del balanceador los toma como comodines.
  special = false
}
