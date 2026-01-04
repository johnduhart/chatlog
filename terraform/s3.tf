# S3 bucket for chat log archives
resource "aws_s3_bucket" "chatlog_archive" {
  bucket = var.s3_bucket_name

  tags = {
    Name        = "chatlog-archive"
    Description = "Storage for Twitch chat log archives"
  }
}

# Block all public access
resource "aws_s3_bucket_public_access_block" "chatlog_archive" {
  bucket = aws_s3_bucket.chatlog_archive.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Enable versioning
resource "aws_s3_bucket_versioning" "chatlog_archive" {
  bucket = aws_s3_bucket.chatlog_archive.id

  versioning_configuration {
    status = var.enable_s3_versioning ? "Enabled" : "Disabled"
  }
}

# Enable server-side encryption (AES256)
resource "aws_s3_bucket_server_side_encryption_configuration" "chatlog_archive" {
  bucket = aws_s3_bucket.chatlog_archive.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true
  }
}

# Lifecycle policy for cost optimization
resource "aws_s3_bucket_lifecycle_configuration" "chatlog_archive" {
  bucket = aws_s3_bucket.chatlog_archive.id

  rule {
    id     = "archive-old-logs"
    status = "Enabled"

    # Transition to Glacier after 90 days (configurable)
    transition {
      days          = var.s3_lifecycle_transition_days
      storage_class = "GLACIER"
    }

    # Optional: Expire after X days (set to 0 to disable)
    dynamic "expiration" {
      for_each = var.s3_lifecycle_expiration_days > 0 ? [1] : []
      content {
        days = var.s3_lifecycle_expiration_days
      }
    }

    # Clean up incomplete multipart uploads after 7 days
    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }

  # Clean up old versions if versioning is enabled
  dynamic "rule" {
    for_each = var.enable_s3_versioning ? [1] : []
    content {
      id     = "expire-old-versions"
      status = "Enabled"

      noncurrent_version_transition {
        noncurrent_days = 30
        storage_class   = "GLACIER"
      }

      noncurrent_version_expiration {
        noncurrent_days = 90
      }
    }
  }
}

# Bucket policy (optional - for additional security)
resource "aws_s3_bucket_policy" "chatlog_archive" {
  bucket = aws_s3_bucket.chatlog_archive.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DenyInsecureTransport"
        Effect = "Deny"
        Principal = "*"
        Action = "s3:*"
        Resource = [
          aws_s3_bucket.chatlog_archive.arn,
          "${aws_s3_bucket.chatlog_archive.arn}/*"
        ]
        Condition = {
          Bool = {
            "aws:SecureTransport" = "false"
          }
        }
      }
    ]
  })
}
