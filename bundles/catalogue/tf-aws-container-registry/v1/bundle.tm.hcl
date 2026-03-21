define bundle metadata {
  class   = "catalogue/tf-aws-container-registry/v1"
  version = "1.0.0"

  name        = "AWS Container Registry"
  description = <<-EOF
    Creates an ECR repository with image scanning, lifecycle policies for
    cost control, replication rules for cross-region availability, and IAM
    policies for pull access from ECS/EKS. Includes a CI/CD-friendly push
    policy for build pipelines.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Repository name (e.g. myorg/api-service)"

    prompt {
      text = "Repository Name"
    }
  }

  input "image_tag_mutability" {
    type        = string
    description = "Whether image tags can be overwritten"
    default     = "IMMUTABLE"

    prompt {
      text    = "Tag Mutability"
      options = ["MUTABLE", "IMMUTABLE"]
    }
  }

  input "scan_on_push" {
    type        = bool
    description = "Enable vulnerability scanning on every push"
    default     = true

    prompt {
      text = "Scan on Push"
    }
  }

  input "max_image_count" {
    type        = number
    description = "Maximum number of images to retain (lifecycle policy)"
    default     = 30

    prompt {
      text = "Max Image Count"
    }
  }

  input "replication_regions" {
    type        = list(string)
    description = "AWS regions to replicate images to (empty for no replication)"
    default     = []

    prompt {
      text = "Replication Regions"
    }
  }

  input "tags" {
    type        = map(string)
    description = "Tags applied to the repository"
    default     = {}

    prompt {
      text = "Tags"
    }
  }
}
