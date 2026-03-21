define bundle metadata {
  class   = "catalogue/tf-aws-monitoring-stack/v1"
  version = "1.0.0"

  name        = "AWS Monitoring Stack"
  description = <<-EOF
    Sets up a centralised monitoring and alerting stack. Provisions CloudWatch
    dashboards, composite alarms, an SNS alert topic with email/Slack
    subscriptions, and a CloudWatch Logs metric filter pipeline for error
    tracking. Designed to be shared across services in a project.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Name for the monitoring stack"

    prompt {
      text = "Stack Name"
    }
  }

  input "alert_email" {
    type        = string
    description = "Email address for alarm notifications"
    default     = ""

    prompt {
      text = "Alert Email"
    }
  }

  input "slack_webhook_url" {
    type        = string
    description = "Slack incoming webhook URL for alarm notifications (optional)"
    default     = ""

    prompt {
      text = "Slack Webhook URL"
    }
  }

  input "log_retention_days" {
    type        = number
    description = "CloudWatch Logs retention period"
    default     = 30

    prompt {
      text    = "Log Retention (days)"
      options = [7, 14, 30, 60, 90, 365]
    }
  }

  input "enable_anomaly_detection" {
    type        = bool
    description = "Enable CloudWatch anomaly detection alarms"
    default     = false

    prompt {
      text = "Enable Anomaly Detection"
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
