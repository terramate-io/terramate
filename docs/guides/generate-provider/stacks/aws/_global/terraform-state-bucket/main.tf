resource "aws_kms_key" "state-bucket-key" {
  description             = "This key is used to encrypt bucket objects"
  deletion_window_in_days = 10
}

resource "aws_s3_bucket" "state-bucket" {
  bucket = "terramate-example-terraform-state-backend"

  object_lock_enabled = true

  tags = {
    Name = "S3 Remote Terraform State Store"
  }
}

resource "aws_s3_bucket_versioning" "state-bucket" {
  bucket = aws_s3_bucket.state-bucket.bucket

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "state-bucket" {
  bucket = aws_s3_bucket.state-bucket.id

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.state-bucket-key.arn
      sse_algorithm     = "aws:kms"
    }
  }
}

resource "aws_dynamodb_table" "terraform-lock" {
  name           = "terraform_state"
  read_capacity  = 5
  write_capacity = 5
  hash_key       = "LockID"
  attribute {
    name = "LockID"
    type = "S"
  }
  tags = {
    "Name" = "DynamoDB Terraform State Lock Table"
  }
}
