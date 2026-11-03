# Entorno de desarrollo en AWS (spec nv-07). Estado local, fuera de Git; sin secretos.
terraform {
  required_version = ">= 1.9"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region  = var.region
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
