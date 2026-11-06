output "instance_id" {
  description = "Instancia EC2 de desarrollo."
  value       = aws_instance.api.id
}

output "public_ip" {
  description = "IP elástica (agrégala al Network Access de MongoDB Atlas)."
  value       = aws_eip.api.public_ip
}

output "site_address" {
  description = "Nombre con el que Caddy pide el certificado."
  value       = local.site_address
}

output "url_https" {
  description = "URL base de la API por HTTPS."
  value       = "https://${local.site_address}"
}

output "ssm_session" {
  description = "Entrar a la instancia sin SSH."
  value       = "aws ssm start-session --target ${aws_instance.api.id} --region ${var.region} --profile ${var.profile}"
}

output "log_group" {
  description = "Grupo de CloudWatch Logs de la API."
  value       = aws_cloudwatch_log_group.api.name
}

output "param_path" {
  description = "Ruta de Parameter Store con los secretos del entorno."
  value       = local.param_path
}

output "verificar" {
  description = "Comprobación de solo lectura del Free Tier y la exposición."
  value       = "./scripts/verificar.sh --profile ${var.profile} --region ${var.region} --instance ${aws_instance.api.id} --url https://${local.site_address}"
}
