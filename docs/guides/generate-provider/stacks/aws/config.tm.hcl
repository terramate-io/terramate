globals "terraform" "providers" "aws" {
  enabled = true
  source  = "hashicorp/aws"
  version = "~> 5.0"
  config = {
    region = "us-east-1"
  }
}
