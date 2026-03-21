environment {
  id          = "dev"
  name        = "Development"
  description = "Development Environment"
}

environment {
  id           = "stg"
  name         = "Staging"
  description  = "Pre-Production Environment: Staging"
  promote_from = "dev"
}

environment {
  id           = "prd"
  name         = "Production"
  description  = "Production Environment"
  promote_from = "stg"
}

environment {
  id          = "shr"
  name        = "Shared"
  description = "Shared Environment for global resources used across environments"
}
