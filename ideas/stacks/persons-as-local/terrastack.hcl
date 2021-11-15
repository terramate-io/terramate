terrastack {
  required_version = "~> 0.1.0"

  name        = "example: persons-as-local"
  description = "Deploy stack terrastack-null-persons as local template."

  # needed for current flink set up and terragrunt stuff
  # will rewrite the module sources while importing:
  # - variales will become locals
  # - sources will be adjusted to match new location if relative source path is used
  clone {
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
