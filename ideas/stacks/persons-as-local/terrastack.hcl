terrastack {
  required_version = "~> 0.1"

  description = "Deploy stack terrastack-null-persons as local template."

  # no label -> clone stack locally
  # needed for current flink set up and terragrunt stuff
  # will rewrite the module sources while importing:
  # - variales will become locals
  # - sources will be adjusted to match new location if relative source path is used
  stack {
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
