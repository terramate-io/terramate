terrstack {
  required_version = "~> 0.1"

  description = "Stack to create a list of persons based on terraform-null-person."

  type = module

  variable "persons" {
    type    = list(object)
    default = []

    attribute "name" {
      type        = string
      description = "The Name of the person."
    }

    attribute "gender" {
      type        = string
      description = "The gender of the person"
    }

    attribute "address" {
      type        = object
      description = "The persons address."

      attribute "street" {
        type        = string
        description = "The street the person lives in."
      }

      attribute "city" {
        type        = string
        description = "The city the person lives in. Default is 'Berlin'"
        default     = "Berlin"
      }
    }
  }
}
