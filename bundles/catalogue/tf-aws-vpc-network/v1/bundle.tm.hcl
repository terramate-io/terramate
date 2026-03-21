define bundle metadata {
  class   = "catalogue/tf-aws-vpc-network/v1"
  version = "1.0.0"

  name        = "AWS VPC Network"
  description = <<-EOF
    Creates a production-grade VPC with public and private subnets across
    multiple availability zones, NAT Gateways, VPC flow logs, a default
    security group, and VPC endpoints for S3 and DynamoDB to reduce data
    transfer costs.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Name for the VPC and all associated resources"

    prompt {
      text = "VPC Name"
    }
  }

  input "cidr_block" {
    type        = string
    description = "Primary CIDR block for the VPC"
    default     = "10.0.0.0/16"

    prompt {
      text = "CIDR Block"
    }
  }

  input "availability_zones" {
    type        = list(string)
    description = "Availability zones for subnet placement"
    default     = ["us-east-1a", "us-east-1b", "us-east-1c"]

    prompt {
      text = "Availability Zones"
    }
  }

  input "nat_gateway_strategy" {
    type        = string
    description = "NAT Gateway deployment strategy"
    default     = "single"

    prompt {
      text    = "NAT Gateway Strategy"
      options = ["none", "single", "one_per_az"]
    }
  }

  input "enable_flow_logs" {
    type        = bool
    description = "Enable VPC flow logs to CloudWatch"
    default     = true

    prompt {
      text = "Enable Flow Logs"
    }
  }

  input "enable_vpc_endpoints" {
    type        = bool
    description = "Create gateway VPC endpoints for S3 and DynamoDB"
    default     = true

    prompt {
      text = "Enable VPC Endpoints"
    }
  }

  input "tags" {
    type        = map(string)
    description = "Tags applied to all resources"
    default     = {}

    prompt {
      text = "Tags"
    }
  }
}
