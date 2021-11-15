terrastack {
  required_version = "~> 0.1.0"

  name        = "example: persons-as-module"
  description = "Deploy stack terrastack-null-persons as terraform module."

  module "persons" {
    source = "../../modules/terrastack-null-persons"

    persons = [
      {
        name   = "marius"
        gender = "male"
        address = {
          street = "mein block 42"
        }
      },
      {
        name   = "elmo"
        gender = "male"
        address = {
          street = "sesamestreet 42"
          city   = "muppetvillage"
        }
      },
    ]
  }
}
