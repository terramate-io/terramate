locals {
  persons_map = { for p in var.persons : p.name => p }
}

module "person" {
  source = "../terraform-null-person"

  for_each = local.persons_map

  name    = each.value.name
  gender  = each.value.gender
  address = merge({city = "Berlin"}, each.value.address)
}
