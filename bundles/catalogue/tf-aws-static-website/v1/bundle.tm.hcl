define bundle metadata {
  class   = "catalogue/tf-aws-static-website/v1"
  version = "1.0.0"

  name        = "AWS Static Website"
  description = <<-EOF
    Deploys a production-ready static website on AWS. Provisions an S3 bucket
    for content, a CloudFront CDN distribution with HTTPS, a Route 53 DNS
    record, and a WAF Web ACL for security. Includes cache invalidation
    support and access logging.
  EOF
}

define bundle {
  input "name" {
    type        = string
    description = "Name for the website (used as prefix for all resources)"

    prompt {
      text = "Website Name"
    }
  }

  input "domain_name" {
    type        = string
    description = "Primary domain name (e.g. www.example.com)"

    prompt {
      text = "Domain Name"
    }
  }

  input "additional_domains" {
    type        = list(string)
    description = "Additional domain aliases for the distribution"
    default     = []

    prompt {
      text = "Additional Domains"
    }
  }

  input "acm_certificate_arn" {
    type        = string
    description = "ARN of the ACM certificate (must be in us-east-1)"

    prompt {
      text = "ACM Certificate ARN"
    }
  }

  input "price_class" {
    type        = string
    description = "CloudFront edge location coverage"
    default     = "PriceClass_100"

    prompt {
      text    = "CDN Price Class"
      options = ["PriceClass_100", "PriceClass_200", "PriceClass_All"]
    }
  }

  input "enable_waf" {
    type        = bool
    description = "Attach a WAF Web ACL with common protections and rate limiting"
    default     = true

    prompt {
      text = "Enable WAF Protection"
    }
  }

  input "index_document" {
    type        = string
    description = "Default index document served by S3"
    default     = "index.html"

    prompt {
      text = "Index Document"
    }
  }

  input "error_document" {
    type        = string
    description = "Custom error page path"
    default     = "error.html"

    prompt {
      text = "Error Document"
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
