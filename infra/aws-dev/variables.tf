variable "region" {
  description = "Región de AWS."
  type        = string
  default     = "us-east-2"
}

variable "profile" {
  description = "Perfil de AWS CLI (aws login)."
  type        = string
  default     = "wherever-dev"
}

variable "project" {
  description = "Nombre del proyecto; prefijo de recursos y de Parameter Store."
  type        = string
  default     = "wherever"
}

variable "environment" {
  description = "Nombre del entorno; prefijo de Parameter Store (/<proyecto>/<entorno>/)."
  type        = string
  default     = "dev"
}

variable "instance_type" {
  description = "Tipo de instancia elegible para el Free Tier (t3.micro o t3.small según la cuenta)."
  type        = string
  default     = "t3.micro"

  validation {
    condition     = contains(["t3.micro", "t3.small", "t4g.micro", "t4g.small"], var.instance_type)
    error_message = "Solo tipos elegibles en el plan gratuito de la cuenta (creada después del 15-jul-2025)."
  }
}

variable "volume_gb" {
  description = "Tamaño del disco raíz gp3 cifrado (8 a 30 GB)."
  type        = number
  default     = 20

  validation {
    condition     = var.volume_gb >= 8 && var.volume_gb <= 30
    error_message = "El disco debe tener entre 8 y 30 GB (Free Tier)."
  }
}

variable "swap_gb" {
  description = "Swap en el disco, como reserva de memoria en 1 GB."
  type        = number
  default     = 2
}

variable "ssh_enabled" {
  description = "Abre el 22 solo desde ssh_cidr. Cerrado por defecto: el acceso es por Systems Manager (nv-12)."
  type        = bool
  default     = false
}

variable "ssh_cidr" {
  description = "IP de quien desarrolla en formato CIDR (p. ej. 203.0.113.10/32). Nunca 0.0.0.0/0."
  type        = string

  validation {
    condition     = can(cidrhost(var.ssh_cidr, 0)) && !startswith(var.ssh_cidr, "0.0.0.0/")
    error_message = "ssh_cidr debe ser un CIDR IPv4 concreto, nunca 0.0.0.0/0."
  }
}

variable "ssh_public_key" {
  description = "Llave pública SSH (contenido de ~/.ssh/<llave>.pub) para el par de llaves de la instancia. La privada nunca sale de tu máquina."
  type        = string
}

variable "app_port" {
  description = "Puerto de la API en la instancia."
  type        = number
  default     = 8080
}

variable "timezone" {
  description = "Zona horaria de las reglas de apagado y encendido."
  type        = string
  default     = "America/Mexico_City"
}

variable "stop_expression" {
  description = "Apagado diario (EventBridge Scheduler)."
  type        = string
  default     = "cron(0 22 * * ? *)"
}

variable "start_expression" {
  description = "Encendido de lunes a viernes."
  type        = string
  default     = "cron(0 8 ? * MON-FRI *)"
}

variable "start_enabled" {
  description = "Si es false, la instancia no se enciende sola (solo se apaga)."
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "Días que CloudWatch conserva los logs de la API."
  type        = number
  default     = 14
}

variable "budget_usd" {
  description = "Tope mensual de gasto para la alerta."
  type        = number
  default     = 1
}

variable "budget_email" {
  description = "Correo que recibe las alertas del presupuesto."
  type        = string

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.budget_email))
    error_message = "budget_email debe ser un correo válido."
  }
}
