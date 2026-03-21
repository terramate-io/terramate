define bundle metadata {
  class   = "catalogue/tf-aws-data-store-env/v1"
  version = "1.0.0"

  name        = "AWS Data Store (per-env)"
  description = <<-EOF
    Provisions a managed data store tier for an application. Creates an RDS
    PostgreSQL instance with automated backups, a Redis ElastiCache cluster
    for caching/sessions, Secrets Manager entries for credentials with
    automatic rotation, and a security group controlling access.
    Right-sized per environment.
  EOF
}

define bundle {
  environments {
    required = true
  }

  alias = "datastore-${bundle.input.name.value}"

  scaffolding {
    name = "${bundle.alias}"
    path = "datastores/${bundle.alias}.tm.yml"
  }

  input "name" {
    type        = string
    description = "Logical name for this data store (used as identifier prefix)"
    immutable   = true

    prompt {
      text = "Data Store Name"
    }
  }

  input "postgres_version" {
    type        = string
    description = "PostgreSQL engine version"
    default     = "16.4"

    prompt {
      text    = "PostgreSQL Version"
      options = ["14.13", "15.8", "16.4"]
    }
  }

  input "db_instance_class" {
    type        = string
    description = "RDS instance type"
    default     = "db.t4g.micro"

    prompt {
      text    = "DB Instance Class"
      options = ["db.t4g.micro", "db.t4g.small", "db.t4g.medium", "db.r7g.large", "db.r7g.xlarge"]
    }
  }

  input "db_storage_gb" {
    type        = number
    description = "Initial database storage allocation in GB"
    default     = 20

    prompt {
      text = "DB Storage (GB)"
    }
  }

  input "multi_az" {
    type        = bool
    description = "Enable Multi-AZ deployment for the database"
    default     = false

    prompt {
      text = "Multi-AZ Database"
    }
  }

  input "cache_node_type" {
    type        = string
    description = "ElastiCache Redis node type"
    default     = "cache.t4g.micro"

    prompt {
      text    = "Cache Node Type"
      options = ["cache.t4g.micro", "cache.t4g.small", "cache.t4g.medium", "cache.r7g.large"]
    }
  }

  input "cache_num_nodes" {
    type        = number
    description = "Number of Redis cache nodes"
    default     = 1

    prompt {
      text = "Cache Node Count"
    }
  }

  input "backup_retention_days" {
    type        = number
    description = "Days to retain automated database backups"
    default     = 7

    prompt {
      text = "Backup Retention (days)"
    }
  }

  input "deletion_protection" {
    type        = bool
    description = "Prevent accidental deletion of the database"
    default     = true

    prompt {
      text = "Deletion Protection"
    }
  }
}
