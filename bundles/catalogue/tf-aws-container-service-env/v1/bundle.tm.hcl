define bundle metadata {
  class   = "catalogue/tf-aws-container-service-env/v1"
  version = "1.0.0"

  name        = "AWS Container Service (per-env)"
  description = <<-EOF
    Deploys a containerised workload on ECS Fargate with full networking.
    Provisions an ECR repository, an ECS service with task definitions, an
    Application Load Balancer with HTTPS, auto-scaling policies, and
    CloudWatch log groups and alarms. Sized per environment.
  EOF
}

define bundle {
  environments {
    required = true
  }

  alias = "svc-${bundle.input.name.value}"

  scaffolding {
    name = "${bundle.alias}"
    path = "services/${bundle.alias}.tm.yml"
  }

  input "name" {
    type        = string
    description = "Service name (used for ECS service, ALB target group, log group, etc.)"
    immutable   = true

    prompt {
      text = "Service Name"
    }
  }

  input "container_image" {
    type        = string
    description = "Docker image URI (e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com/app:latest)"

    prompt {
      text = "Container Image"
    }
  }

  input "container_port" {
    type        = number
    description = "Port the container listens on"
    default     = 8080

    prompt {
      text = "Container Port"
    }
  }

  input "cpu" {
    type        = number
    description = "CPU units for the Fargate task (256 = 0.25 vCPU)"
    default     = 256

    prompt {
      text    = "CPU Units"
      options = [256, 512, 1024, 2048, 4096]
    }
  }

  input "memory" {
    type        = number
    description = "Memory in MiB for the Fargate task"
    default     = 512

    prompt {
      text    = "Memory (MiB)"
      options = [512, 1024, 2048, 4096, 8192]
    }
  }

  input "desired_count" {
    type        = number
    description = "Baseline number of running tasks"
    default     = 2

    prompt {
      text = "Desired Count"
    }
  }

  input "max_count" {
    type        = number
    description = "Maximum tasks for auto-scaling"
    default     = 10

    prompt {
      text = "Max Task Count"
    }
  }

  input "health_check_path" {
    type        = string
    description = "HTTP path for ALB health checks"
    default     = "/health"

    prompt {
      text = "Health Check Path"
    }
  }

  input "environment_variables" {
    type        = map(string)
    description = "Environment variables injected into the container"
    default     = {}

    prompt {
      text = "Environment Variables"
    }
  }
}
