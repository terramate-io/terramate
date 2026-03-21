define bundle metadata {
  class   = "catalogue/tf-aws-batch-processor-env/v1"
  version = "1.0.0"

  name        = "AWS Batch Processor (per-env)"
  description = <<-EOF
    Deploys a background job processing system per environment. Provisions an
    SQS queue with DLQ, a Lambda or ECS-based consumer, a DynamoDB table for
    job state tracking, CloudWatch alarms for queue depth, and IAM roles.
    Supports scheduled (cron) and event-triggered execution.
  EOF
}

define bundle {
  environments {
    required = true
  }

  alias = "batch-${bundle.input.name.value}"

  scaffolding {
    name = "${bundle.alias}"
    path = "processors/${bundle.alias}.tm.yml"
  }

  input "name" {
    type        = string
    description = "Processor name (used for queue, functions, and log groups)"
    immutable   = true

    prompt {
      text = "Processor Name"
    }
  }

  input "compute_type" {
    type        = string
    description = "Compute backend for processing jobs"
    default     = "lambda"

    prompt {
      text    = "Compute Type"
      options = ["lambda", "ecs"]
    }
  }

  input "concurrency" {
    type        = number
    description = "Maximum concurrent job executions"
    default     = 5

    prompt {
      text = "Max Concurrency"
    }
  }

  input "timeout_seconds" {
    type        = number
    description = "Maximum execution time per job in seconds"
    default     = 300

    prompt {
      text = "Job Timeout (seconds)"
    }
  }

  input "schedule_expression" {
    type        = string
    description = "EventBridge schedule expression for cron jobs (leave empty for event-only)"
    default     = ""

    prompt {
      text = "Schedule Expression"
    }
  }

  input "dlq_alarm_threshold" {
    type        = number
    description = "DLQ message count that triggers a CloudWatch alarm"
    default     = 10

    prompt {
      text = "DLQ Alarm Threshold"
    }
  }

  input "environment_variables" {
    type        = map(string)
    description = "Environment variables for the processor"
    default     = {}

    prompt {
      text = "Environment Variables"
    }
  }
}
