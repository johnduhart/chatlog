# Data source to get Fly.io OIDC provider's thumbprint
# Fly.io OIDC issuer: https://oidc.fly.io/<org-slug>
data "tls_certificate" "flyio_oidc" {
  url = "https://oidc.fly.io/${var.flyio_org}"
}

# Create IAM OIDC Identity Provider for Fly.io
resource "aws_iam_openid_connect_provider" "flyio" {
  url = "https://oidc.fly.io/${var.flyio_org}"

  # AWS requires sts.amazonaws.com as the audience
  client_id_list = [
    "sts.amazonaws.com"
  ]

  # TLS certificate thumbprint from Fly.io's OIDC endpoint
  thumbprint_list = [
    data.tls_certificate.flyio_oidc.certificates[0].sha1_fingerprint
  ]

  tags = {
    Name        = "flyio-oidc-provider"
    Description = "OIDC provider for Fly.io authentication"
  }
}

# IAM Role that Fly.io application will assume
resource "aws_iam_role" "flyio_chatlog" {
  name        = "flyio-chatlog-s3-access"
  description = "Role for Fly.io chatlog application to access S3"

  # Trust policy allowing Fly.io to assume this role
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = aws_iam_openid_connect_provider.flyio.arn
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            # Ensure the audience is AWS STS
            "oidc.fly.io/${var.flyio_org}:aud" = "sts.amazonaws.com"
          }
          StringLike = {
            # Restrict to specific Fly.io app (format: org:app:machine)
            # Wildcard allows any machine in the app
            "oidc.fly.io/${var.flyio_org}:sub" = "${var.flyio_org}:${var.flyio_app_name}:*"
          }
        }
      }
    ]
  })

  # Maximum session duration (1 hour default, can be up to 12 hours)
  max_session_duration = 3600

  tags = {
    Name        = "flyio-chatlog-role"
    Description = "IAM role for Fly.io chatlog application"
  }
}

# Attach S3 access policy to the role
resource "aws_iam_role_policy_attachment" "flyio_chatlog_s3" {
  role       = aws_iam_role.flyio_chatlog.name
  policy_arn = aws_iam_policy.s3_chatlog_access.arn
}
