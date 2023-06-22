globals "terraform" "providers" "aws" {
  enabled = true
  source  = "hashicorp/aws"
  version = "~> 5.0"
  config = {
    region = "us-east-1"
  }
}

globals "terraform" "providers" "aws.west" {
  config = {
    region = "us-west-1"
  }
}
