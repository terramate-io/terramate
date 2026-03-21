define bundle metadata {
  class   = "catalogue/tf-aws-secure-bucket/v1"
  version = "1.0.0"

  name        = "AWS Secure Bucket"
  description = <<-EOF
    Creates a hardened S3 bucket following AWS security best practices.
    Enables versioning, server-side encryption, access logging, public access
    blocking, and configurable lifecycle policies for cost optimisation.
    Suitable for data lakes, backups, and compliance-sensitive workloads.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Globally unique bucket name"

    prompt {
      text = "Bucket Name"
    }
  }

  input "purpose" {
    type        = string
    description = "Primary purpose of this bucket"
    default     = "general"

    prompt {
      text    = "Bucket Purpose"
      options = ["general", "data-lake", "backups", "logs", "artifacts"]
    }
  }

  input "encryption" {
    type        = string
    description = "Encryption method"
    default     = "AES256"

    prompt {
      text    = "Encryption"
      options = ["AES256", "aws:kms"]
    }
  }

  input "enable_access_logging" {
    type        = bool
    description = "Enable S3 access logging to a separate bucket"
    default     = true

    prompt {
      text = "Enable Access Logging"
    }
  }

  input "lifecycle_ia_days" {
    type        = number
    description = "Days before transitioning objects to Infrequent Access (0 to skip)"
    default     = 90

    prompt {
      text = "Transition to IA (days)"
    }
  }

  input "lifecycle_glacier_days" {
    type        = number
    description = "Days before transitioning objects to Glacier (0 to skip)"
    default     = 365

    prompt {
      text = "Transition to Glacier (days)"
    }
  }

  input "tags" {
    type        = map(string)
    description = "Tags applied to the bucket"
    default     = {}

    prompt {
      text = "Tags"
    }
  }
}
