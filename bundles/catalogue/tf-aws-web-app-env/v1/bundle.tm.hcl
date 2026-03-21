define bundle metadata {
  class   = "catalogue/tf-aws-web-app-env/v1"
  version = "1.0.0"

  name        = "AWS Web Application (per-env)"
  description = <<-EOF
    Full-stack web application deployment per environment. Provisions an ECS
    Fargate service behind an ALB with HTTPS, a CloudFront distribution for
    static assets, an RDS PostgreSQL database, a Redis cache, and Secrets
    Manager entries for credentials. All components are right-sized per
    environment.
  EOF
}

define bundle {
  environments {
    required = true
  }

  alias = "webapp-${bundle.input.name.value}"

  scaffolding {
    name = "${bundle.alias}"
    path = "apps/${bundle.alias}.tm.yml"
  }

  input "name" {
    type        = string
    description = "Application name (prefix for all resources)"
    immutable   = true

    prompt {
      text = "Application Name"
    }
  }

  input "container_image" {
    type        = string
    description = "Docker image for the application"

    prompt {
      text = "Container Image"
    }
  }

  input "domain_name" {
    type        = string
    description = "Domain name for the application"
    default     = ""

    prompt {
      text = "Domain Name"
    }
  }

  input "instance_size" {
    type        = string
    description = "T-shirt size controlling compute and database resources"
    default     = "small"

    prompt {
      text    = "Instance Size"
      options = ["small", "medium", "large", "xlarge"]
    }
  }

  input "desired_count" {
    type        = number
    description = "Number of application instances"
    default     = 2

    prompt {
      text = "Instance Count"
    }
  }

  input "enable_cdn" {
    type        = bool
    description = "Put a CloudFront CDN in front of the application"
    default     = true

    prompt {
      text = "Enable CDN"
    }
  }

  input "enable_cache" {
    type        = bool
    description = "Provision a Redis cache cluster"
    default     = true

    prompt {
      text = "Enable Cache"
    }
  }

  input "db_multi_az" {
    type        = bool
    description = "Enable Multi-AZ for the database"
    default     = false

    prompt {
      text = "Database Multi-AZ"
    }
  }

  input "environment_variables" {
    type        = map(string)
    description = "Environment variables for the application"
    default     = {}

    prompt {
      text = "Environment Variables"
    }
  }
}
