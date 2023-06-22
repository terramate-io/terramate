// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

terraform {
  backend "s3" {
    bucket         = "terramate-example-terraform-state-backend"
    dynamodb_table = "terraform_state"
    encrypt        = true
    key            = "terraform/stacks/by-id/e25a4be6-47c0-4952-a769-1e4ecf307592/terraform.tfstate"
    region         = "us-east-1"
  }
}
