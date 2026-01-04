variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (e.g., production, staging)"
  type        = string
  default     = "production"
}

variable "flyio_org" {
  description = "Fly.io organization name"
  type        = string
  default     = "personal"
}

variable "flyio_app_name" {
  description = "Fly.io application name"
  type        = string
  default     = "chatlog"
}

variable "s3_bucket_name" {
  description = "S3 bucket name for chat log archives"
  type        = string
  default     = "chatlog-archive"
}

variable "s3_lifecycle_transition_days" {
  description = "Days before transitioning objects to Glacier"
  type        = number
  default     = 90
}

variable "s3_lifecycle_expiration_days" {
  description = "Days before expiring objects (0 = never expire)"
  type        = number
  default     = 0
}

variable "enable_s3_versioning" {
  description = "Enable S3 bucket versioning"
  type        = bool
  default     = true
}
