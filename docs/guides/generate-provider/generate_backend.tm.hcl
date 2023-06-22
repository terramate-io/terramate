generate_hcl "_terramate_generated_backend.tf" {
  condition = tm_try(global.terraform.backend.enabled, true)

  content {
    terraform {
      backend "s3" {
        region         = "us-east-1"
        bucket         = "terramate-example-terraform-state-backend"
        key            = "terraform/stacks/by-id/${terramate.stack.id}/terraform.tfstate"
        encrypt        = true
        dynamodb_table = "terraform_state"
      }
    }
  }
}
