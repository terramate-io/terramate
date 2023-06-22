globals "terraform" {
  version = "1.5.1"
}


# Will be added to all stacks
globals "terraform" "providers" "random" {
  source  = "hashicorp/random"
  version = "~> 3.5"
  enabled = true
}
