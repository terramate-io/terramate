generate_hcl "_terramate_generated_providers.tf" {

  lets {
    required_providers = { for k, v in tm_try(global.terraform.providers, {}) :
      k => {
        source  = v.source
        version = v.version
      } if tm_try(v.enabled, true)
    }
    providers = { for k, v in tm_try(global.terraform.providers, {}) :
      k => v.config if tm_try(v.enabled, true) && tm_can(v.config)
    }
  }

  content {
    # terraform version constraints
    terraform {
      required_version = tm_try(global.terraform.version, "~> 1.5")
    }

    # Provider version constraints
    terraform {
      tm_dynamic "required_providers" {
        attributes = let.required_providers
      }
    }

    tm_dynamic "provider" {
      for_each   = let.providers
      labels     = [provider.key]
      attributes = provider.value
    }
  }
}
