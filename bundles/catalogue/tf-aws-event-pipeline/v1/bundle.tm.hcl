define bundle metadata {
  class   = "catalogue/tf-aws-event-pipeline/v1"
  version = "1.0.0"

  name        = "AWS Event Pipeline"
  description = <<-EOF
    Creates an event-driven processing pipeline. Provisions an SNS topic for
    fan-out, SQS queues with dead-letter queues for reliable consumption,
    Lambda functions for processing, and CloudWatch alarms for DLQ depth
    monitoring. Designed for decoupled microservice communication.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Pipeline name (used as prefix for all resources)"

    prompt {
      text = "Pipeline Name"
    }
  }

  input "num_consumers" {
    type        = number
    description = "Number of independent SQS consumer queues to create"
    default     = 1

    prompt {
      text = "Number of Consumers"
    }
  }

  input "fifo" {
    type        = bool
    description = "Use FIFO queues and topic for strict ordering"
    default     = false

    prompt {
      text = "FIFO (Ordered)"
    }
  }

  input "message_retention_days" {
    type        = number
    description = "Number of days messages are retained in the queue"
    default     = 4

    prompt {
      text    = "Message Retention (days)"
      options = [1, 4, 7, 14]
    }
  }

  input "dlq_max_receives" {
    type        = number
    description = "Number of receive attempts before a message moves to the DLQ"
    default     = 3

    prompt {
      text = "Max Receive Attempts"
    }
  }

  input "enable_lambda_consumer" {
    type        = bool
    description = "Provision a Lambda function as the primary queue consumer"
    default     = false

    prompt {
      text = "Include Lambda Consumer"
    }
  }

  input "lambda_runtime" {
    type        = string
    description = "Runtime for the Lambda consumer (if enabled)"
    default     = "python3.12"

    prompt {
      text    = "Lambda Runtime"
      options = ["python3.12", "python3.11", "nodejs20.x", "nodejs22.x"]
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
