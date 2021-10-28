terrstack {
  required_version = "~> 0.1"

  # with label -> create a module entry locally (recommended way)
  # needed for current flink set up and terragrunt stuff
  stack "persons" {
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
