// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

terraform {
  backend "s3" {
    bucket         = "terramate-example-terraform-state-backend"
    dynamodb_table = "terraform_state"
    encrypt        = true
    key            = "terraform/stacks/by-id/550f4ffd-9e58-4599-b9d4-d730286d79ec/terraform.tfstate"
    region         = "us-east-1"
  }
}
