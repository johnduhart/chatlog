output "oidc_provider_arn" {
  description = "ARN of the Fly.io OIDC provider"
  value       = aws_iam_openid_connect_provider.flyio.arn
}

output "iam_role_arn" {
  description = "ARN of the IAM role for Fly.io to assume"
  value       = aws_iam_role.flyio_chatlog.arn
}

output "iam_role_name" {
  description = "Name of the IAM role"
  value       = aws_iam_role.flyio_chatlog.name
}

output "s3_bucket_name" {
  description = "Name of the S3 bucket for chat logs"
  value       = aws_s3_bucket.chatlog_archive.id
}

output "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.chatlog_archive.arn
}

output "s3_bucket_region" {
  description = "Region of the S3 bucket"
  value       = aws_s3_bucket.chatlog_archive.region
}

# Critical output for application configuration
output "flyio_environment_variables" {
  description = "Environment variables to set in Fly.io"
  value = {
    AWS_REGION   = var.aws_region
    AWS_ROLE_ARN = aws_iam_role.flyio_chatlog.arn
    S3_BUCKET    = aws_s3_bucket.chatlog_archive.id
  }
}
