# IAM Policy for S3 access (least privilege)
resource "aws_iam_policy" "s3_chatlog_access" {
  name        = "chatlog-s3-access"
  description = "Least privilege S3 access for chatlog application"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "ListBucket"
        Effect = "Allow"
        Action = [
          "s3:ListBucket",
          "s3:GetBucketLocation"
        ]
        Resource = aws_s3_bucket.chatlog_archive.arn
      },
      {
        Sid    = "ReadWriteObjects"
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:GetObject",
          "s3:DeleteObject",
          "s3:GetObjectVersion"
        ]
        Resource = "${aws_s3_bucket.chatlog_archive.arn}/*"
      }
    ]
  })

  tags = {
    Name        = "chatlog-s3-policy"
    Description = "S3 access policy for chatlog uploads"
  }
}
