// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

terraform {
  backend "s3" {
    bucket         = "terramate-example-terraform-state-backend"
    dynamodb_table = "terraform_state"
    encrypt        = true
    key            = "terraform/stacks/by-id/05cb70d5-a2a6-4b36-bf20-2e01ade1070e/terraform.tfstate"
    region         = "us-east-1"
  }
}
