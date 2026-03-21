define bundle metadata {
  class   = "catalogue/tf-aws-ci-cd-pipeline/v1"
  version = "1.0.0"

  name        = "AWS CI/CD Pipeline"
  description = <<-EOF
    Creates a full CI/CD pipeline using CodePipeline and CodeBuild. Provisions
    a CodePipeline connected to a source repository, CodeBuild projects for
    build and test stages, an artifact S3 bucket, IAM roles, and SNS
    notifications for pipeline state changes.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Pipeline name"

    prompt {
      text = "Pipeline Name"
    }
  }

  input "repository_url" {
    type        = string
    description = "Source repository URL (CodeCommit, GitHub, or S3)"

    prompt {
      text = "Repository URL"
    }
  }

  input "branch" {
    type        = string
    description = "Branch that triggers the pipeline"
    default     = "main"

    prompt {
      text = "Branch"
    }
  }

  input "build_image" {
    type        = string
    description = "CodeBuild Docker image for the build environment"
    default     = "aws/codebuild/amazonlinux2-x86_64-standard:5.0"

    prompt {
      text    = "Build Image"
      options = [
        "aws/codebuild/amazonlinux2-x86_64-standard:5.0",
        "aws/codebuild/amazonlinux2-aarch64-standard:3.0",
        "aws/codebuild/standard:7.0",
      ]
    }
  }

  input "compute_type" {
    type        = string
    description = "CodeBuild compute instance size"
    default     = "BUILD_GENERAL1_SMALL"

    prompt {
      text    = "Compute Type"
      options = ["BUILD_GENERAL1_SMALL", "BUILD_GENERAL1_MEDIUM", "BUILD_GENERAL1_LARGE"]
    }
  }

  input "enable_test_stage" {
    type        = bool
    description = "Add a separate test stage after build"
    default     = true

    prompt {
      text = "Include Test Stage"
    }
  }

  input "notification_email" {
    type        = string
    description = "Email for pipeline failure notifications (optional)"
    default     = ""

    prompt {
      text = "Notification Email"
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
