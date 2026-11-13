output "instance_ids" {
  description = "Instancias de la API por nodo."
  value       = { for node, instance in aws_instance.api : node => instance.id }
}

output "target_group_arn" {
  description = "Grupo de destino del balanceador."
  value       = aws_lb_target_group.api.arn
}

output "cloudfront_url" {
  description = "URL pública de la API."
  value       = "https://${aws_cloudfront_distribution.api.domain_name}"
}

output "imagenes_bucket" {
  description = "Bucket privado de imágenes (nv-44); IMAGENES_BUCKET en Parameter Store."
  value       = aws_s3_bucket.imagenes.id
}

output "imagenes_url" {
  description = "URL de CloudFront de las imágenes (nv-44); IMAGENES_URL en Parameter Store. Cambia si se recrea."
  value       = "https://${aws_cloudfront_distribution.imagenes.domain_name}"
}

output "alb_dns" {
  description = "Balanceador; no responde directo desde internet."
  value       = aws_lb.api.dns_name
}

output "public_ips" {
  description = "IP elásticas por nodo (agrégalas al Network Access de MongoDB Atlas)."
  value       = { for node, eip in aws_eip.api : node => eip.public_ip }
}

output "ssm_session" {
  description = "Entrar a la instancia sin SSH."
  value       = "aws ssm start-session --target ${aws_instance.api["a"].id} --region ${var.region} --profile ${var.profile}"
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
  value       = "./scripts/verificar.sh --profile ${var.profile} --region ${var.region} --instance ${aws_instance.api["a"].id} --url https://${aws_cloudfront_distribution.api.domain_name} --imagenes https://${aws_cloudfront_distribution.imagenes.domain_name}"
}
