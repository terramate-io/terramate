define bundle metadata {
  class   = "catalogue/tf-aws-secret/v1"
  version = "1.0.0"

  name        = "AWS Secret"
  description = <<-EOF
    Creates a Secrets Manager secret with KMS encryption, automatic rotation
    via a Lambda function, resource policies for cross-account access, and
    CloudWatch alarms for rotation failures. Suitable for database
    credentials, API keys, and service tokens.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Secret name (path-style, e.g. myapp/database/credentials)"

    prompt {
      text = "Secret Name"
    }
  }

  input "description" {
    type        = string
    description = "Human-readable description"
    default     = ""

    prompt {
      text = "Description"
    }
  }

  input "secret_type" {
    type        = string
    description = "Type of secret (affects rotation configuration)"
    default     = "generic"

    prompt {
      text    = "Secret Type"
      options = ["generic", "rds-credentials", "api-key"]
    }
  }

  input "enable_rotation" {
    type        = bool
    description = "Enable automatic secret rotation"
    default     = false

    prompt {
      text = "Enable Rotation"
    }
  }

  input "rotation_days" {
    type        = number
    description = "Days between automatic rotations"
    default     = 30

    prompt {
      text    = "Rotation Interval (days)"
      options = [7, 14, 30, 60, 90]
    }
  }

  input "recovery_window_days" {
    type        = number
    description = "Days before a deleted secret is permanently removed"
    default     = 30

    prompt {
      text    = "Recovery Window (days)"
      options = [0, 7, 14, 30]
    }
  }

  input "tags" {
    type        = map(string)
    description = "Tags applied to the secret"
    default     = {}

    prompt {
      text = "Tags"
    }
  }
}
