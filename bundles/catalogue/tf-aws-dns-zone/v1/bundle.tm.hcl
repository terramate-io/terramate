define bundle metadata {
  class   = "catalogue/tf-aws-dns-zone/v1"
  version = "1.0.0"

  name        = "AWS DNS Zone"
  description = <<-EOF
    Creates a Route 53 hosted zone with DNSSEC signing, health checks, and
    optional delegation from a parent zone. Supports both public zones for
    internet-facing services and private zones for internal service discovery
    within a VPC.
  EOF
}

define bundle {
  input "domain_name" {
    type        = string
    description = "Domain name for the hosted zone"

    prompt {
      text = "Domain Name"
    }
  }

  input "private_zone" {
    type        = bool
    description = "Create a private zone (for VPC-internal DNS)"
    default     = false

    prompt {
      text = "Private Zone"
    }
  }

  input "enable_dnssec" {
    type        = bool
    description = "Enable DNSSEC signing for the zone"
    default     = false

    prompt {
      text = "Enable DNSSEC"
    }
  }

  input "comment" {
    type        = string
    description = "Description of the zone's purpose"
    default     = "Managed by Terramate"

    prompt {
      text = "Comment"
    }
  }

  input "tags" {
    type        = map(string)
    description = "Tags applied to the hosted zone"
    default     = {}

    prompt {
      text = "Tags"
    }
  }
}
