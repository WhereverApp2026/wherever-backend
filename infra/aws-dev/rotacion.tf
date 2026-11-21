# Aviso de secretos sin rotar (plan de seguridad, control 14 de docs/seguridad.md).
# scripts/rotar-secretos.sh guarda cada secreto como parámetro avanzado con NoChangeNotification
# (90 días los propios, 180 los de terceros). Al vencer, Parameter Store emite un evento y esta
# regla lo manda al mismo correo de las alarmas. Costo: US$0,05 al mes por parámetro avanzado.

resource "aws_cloudwatch_event_rule" "secretos" {
  name        = "${local.name}-secretos-sin-rotar"
  description = "Un secreto de ${local.param_path} pasó su plazo sin rotarse"
  event_pattern = jsonencode({
    source        = ["aws.ssm"]
    "detail-type" = ["Parameter Store Policy Action"]
    detail = {
      "parameter-name" = [{ prefix = local.param_path }]
      "policy-type"    = ["NoChangeNotification"]
    }
  })
}

resource "aws_cloudwatch_event_target" "secretos" {
  rule      = aws_cloudwatch_event_rule.secretos.name
  target_id = "correo-de-alarmas"
  arn       = aws_sns_topic.alarms.arn

  input_transformer {
    input_paths = {
      parametro = "$.detail.parameter-name"
      cuando    = "$.time"
    }
    input_template = "\"<parametro> no se ha rotado en su plazo (<cuando>). Rótalo con infra/aws-dev/scripts/rotar-secretos.sh.\""
  }
}

# Con política propia el tema pierde la que AWS pone por defecto: se repiten la de la cuenta y la
# de CloudWatch (alarmas) y se agrega EventBridge, solo para esta regla.
data "aws_iam_policy_document" "alarms_topic" {
  statement {
    sid = "LaCuentaAdministra"
    actions = [
      "SNS:GetTopicAttributes", "SNS:SetTopicAttributes", "SNS:AddPermission", "SNS:RemovePermission",
      "SNS:DeleteTopic", "SNS:Subscribe", "SNS:ListSubscriptionsByTopic", "SNS:Publish",
    ]
    resources = [aws_sns_topic.alarms.arn]
    principals {
      type        = "AWS"
      identifiers = ["*"]
    }
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceOwner"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
  statement {
    sid       = "AlarmasDeCloudWatch"
    actions   = ["SNS:Publish"]
    resources = [aws_sns_topic.alarms.arn]
    principals {
      type        = "Service"
      identifiers = ["cloudwatch.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
  statement {
    sid       = "SecretosSinRotar"
    actions   = ["SNS:Publish"]
    resources = [aws_sns_topic.alarms.arn]
    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_cloudwatch_event_rule.secretos.arn]
    }
  }
}

resource "aws_sns_topic_policy" "alarms" {
  arn    = aws_sns_topic.alarms.arn
  policy = data.aws_iam_policy_document.alarms_topic.json
}
