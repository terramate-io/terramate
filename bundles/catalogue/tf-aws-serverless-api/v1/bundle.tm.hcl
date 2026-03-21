define bundle metadata {
  class   = "catalogue/tf-aws-serverless-api/v1"
  version = "1.0.0"

  name        = "AWS Serverless API"
  description = <<-EOF
    Deploys a serverless REST API backed by Lambda functions and API Gateway.
    Provisions Lambda functions, an API Gateway HTTP API with custom domain,
    IAM execution roles, CloudWatch log groups, and X-Ray tracing. Includes
    a DynamoDB table for application state.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "API name (used for Lambda function prefix, API Gateway, etc.)"

    prompt {
      text = "API Name"
    }
  }

  input "runtime" {
    type        = string
    description = "Lambda runtime for the API handlers"
    default     = "python3.12"

    prompt {
      text    = "Runtime"
      options = ["python3.12", "python3.11", "nodejs20.x", "nodejs22.x", "provided.al2023"]
    }
  }

  input "memory_size" {
    type        = number
    description = "Memory in MB for each Lambda function"
    default     = 256

    prompt {
      text = "Lambda Memory (MB)"
    }
  }

  input "timeout" {
    type        = number
    description = "Maximum execution time per request in seconds"
    default     = 30

    prompt {
      text = "Request Timeout (seconds)"
    }
  }

  input "custom_domain" {
    type        = string
    description = "Custom domain name for the API (e.g. api.example.com)"
    default     = ""

    prompt {
      text = "Custom Domain"
    }
  }

  input "enable_xray" {
    type        = bool
    description = "Enable AWS X-Ray distributed tracing"
    default     = true

    prompt {
      text = "Enable X-Ray Tracing"
    }
  }

  input "throttle_rate_limit" {
    type        = number
    description = "Steady-state request rate limit (requests per second)"
    default     = 1000

    prompt {
      text = "Rate Limit (req/s)"
    }
  }

  input "environment_variables" {
    type        = map(string)
    description = "Environment variables for the Lambda functions"
    default     = {}

    prompt {
      text = "Environment Variables"
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
