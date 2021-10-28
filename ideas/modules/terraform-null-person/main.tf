variable "name" {
  type = string
}

variable "gender" {
  type = string
}

variable "address" {
  type    = any
}

resource "null_resource" "person" {
  triggers = {
    name           = var.name
    gender         = var.gender
    address_street = var.address.street
    address_city   = var.address.city
  }
}
