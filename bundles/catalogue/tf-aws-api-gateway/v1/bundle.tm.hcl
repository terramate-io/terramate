define bundle metadata {
  class   = "catalogue/tf-aws-api-gateway/v1"
  version = "1.0.0"

  name        = "AWS API Gateway"
  description = <<-EOF
    Creates an API Gateway HTTP API with custom domain, WAF protection,
    access logging, and request throttling. Can front any backend (ALB,
    Lambda, or HTTP endpoint). Includes a Route 53 DNS alias record and
    ACM certificate validation.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "API Gateway name"

    prompt {
      text = "Gateway Name"
    }
  }

  input "domain_name" {
    type        = string
    description = "Custom domain name (e.g. api.example.com)"

    prompt {
      text = "Domain Name"
    }
  }

  input "backend_type" {
    type        = string
    description = "Type of backend integration"
    default     = "alb"

    prompt {
      text    = "Backend Type"
      options = ["alb", "lambda", "http"]
    }
  }

  input "backend_url" {
    type        = string
    description = "URL or ARN of the backend to route traffic to"

    prompt {
      text = "Backend URL / ARN"
    }
  }

  input "cors_origins" {
    type        = list(string)
    description = "Allowed CORS origins"
    default     = ["*"]

    prompt {
      text = "CORS Origins"
    }
  }

  input "throttle_rate" {
    type        = number
    description = "Steady-state request rate limit per second"
    default     = 1000

    prompt {
      text = "Rate Limit (req/s)"
    }
  }

  input "throttle_burst" {
    type        = number
    description = "Maximum burst request capacity"
    default     = 2000

    prompt {
      text = "Burst Limit"
    }
  }

  input "enable_waf" {
    type        = bool
    description = "Attach a WAF Web ACL with managed rules"
    default     = true

    prompt {
      text = "Enable WAF"
    }
  }

  input "tags" {
    type        = map(string)
    description = "Tags applied to all resources"
    default     = {}

    prompt {
      text = "Tags"
    }
  }
}
