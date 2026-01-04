# Fly.io OIDC Deployment Guide

This guide walks you through deploying the Fly.io OIDC federation setup for secure AWS S3 access without long-lived credentials.

## Prerequisites

- AWS CLI configured with appropriate credentials
- Terraform >= 1.5.0 installed
- Fly CLI installed and authenticated
- Access to modify IAM in your AWS account

## Step 1: Deploy Terraform Infrastructure

```bash
cd /home/john/src/chatlog/terraform

# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Apply the infrastructure
terraform apply
```

When prompted, type `yes` to confirm.

**Expected Output:**
- IAM OIDC provider created
- IAM role `flyio-chatlog-s3-access` created
- S3 bucket `chatlog-archive` created with encryption and lifecycle policies
- IAM policy attached to role

## Step 2: Capture Critical Outputs

After Terraform completes, save the IAM role ARN:

```bash
# Get the role ARN
terraform output -raw iam_role_arn

# Example output: arn:aws:iam::123456789012:role/flyio-chatlog-s3-access
```

Copy this ARN - you'll need it for Fly.io configuration.

## Step 3: Update Go Dependencies

Install the new AWS SDK dependencies:

```bash
cd /home/john/src/chatlog
go get github.com/aws/aws-sdk-go-v2/service/sts
go get github.com/aws/aws-sdk-go-v2/credentials/stscreds
go mod tidy
```

## Step 4: Customize Configuration File

**IMPORTANT:** Edit `config.yaml` to configure your Twitch settings before deploying:

```yaml
twitch:
  username: your_bot_username  # CUSTOMIZE THIS
  oauth: ""                     # Set via TWITCH_OAUTH env var (Fly secret)
  channels:                     # CUSTOMIZE THIS - list your channels
    - channel1
    - channel2

s3:
  bucket: "chatlog-archive"
  region: "us-east-1"
  role_arn: ""                  # Set via AWS_ROLE_ARN env var (Fly secret)
  # No static credentials - using OIDC authentication
```

**Note:** The config file is baked into the Docker image during build, so customize it before running `fly deploy`.

## Step 5: Configure Fly.io Secrets

Set the required secrets (IAM role ARN and Twitch OAuth token):

```bash
cd /home/john/src/chatlog

# Set the IAM role ARN (use the output from Step 2)
fly secrets set AWS_ROLE_ARN="arn:aws:iam::123456789012:role/flyio-chatlog-s3-access"

# Set your Twitch OAuth token (get from https://twitchapps.com/tmi/)
fly secrets set TWITCH_OAUTH="oauth:your_token_here"
```

**Do NOT remove old S3 secrets yet** - wait until after verification.

## Step 6: Deploy to Fly.io

Deploy the updated application:

```bash
cd /home/john/src/chatlog
fly deploy
```

## Step 7: Verify OIDC API Availability

Once deployed, verify the Fly.io OIDC API is accessible:

```bash
fly ssh console

# Inside the VM, test the OIDC token endpoint
curl --unix-socket /.fly/api -X POST "http://localhost/v1/tokens/oidc" --data '{"aud":"sts.amazonaws.com"}'

# Should display a JWT token (long string)
# Press Ctrl+D to exit
```

If the API returns a token, OIDC is working correctly.

## Step 8: Monitor Logs

Watch the application logs for successful startup and S3 uploads:

```bash
fly logs

# Look for:
# - "Using OIDC authentication with role: arn:aws:iam::..."
# - "Scanning /app/data for existing files to upload..."
# - "Found X existing file(s) to upload" (if files exist from previous runs)
# - "Successfully uploaded ... to s3://chatlog-archive/..."
# - NO errors about credentials or access denied
```

**Note:** On startup, the application automatically scans the data directory for any existing `.jsonl` files and queues them for upload. This ensures that files created before a restart are not lost.

## Step 9: Verify S3 Uploads

Check that files are being uploaded to S3:

```bash
# List files in the S3 bucket
aws s3 ls s3://chatlog-archive/ --recursive

# You should see files organized by date:
# 2025/12/30/twitch/channel_name/twitch_channel_20251230_1234.jsonl
```

## Step 10: Cleanup Old Secrets

Once you've confirmed OIDC is working (logs show successful uploads), remove the old static credentials:

```bash
fly secrets unset S3_ACCESS_KEY_ID S3_SECRET_ACCESS_KEY
```

## Troubleshooting

### Error: "request token: dial unix /.fly/api: no such file or directory"

**Cause:** Fly.io OIDC API socket not available (not running on Fly.io)

**Solution:**
```bash
fly ssh console
ls -la /.fly/api
# Check if socket exists
```

If missing, ensure you're running on Fly.io infrastructure. The OIDC API is only available on Fly Machines.

### Error: "AssumeRoleWithWebIdentity failed"

**Cause:** IAM role trust policy issue or incorrect audience/subject

**Solution:**
```bash
# Verify IAM role trust policy
aws iam get-role --role-name flyio-chatlog-s3-access

# Check the trust policy conditions match:
# - Issuer: oidc.fly.io/personal
# - Audience (aud): sts.amazonaws.com
# - Subject (sub): personal:chatlog:* (org:app:machine pattern)
```

### Error: "Access Denied" on S3 operations

**Cause:** IAM policy insufficient permissions

**Solution:**
```bash
# Check attached policies
aws iam list-attached-role-policies --role-name flyio-chatlog-s3-access

# View policy document
aws iam get-policy-version \
  --policy-arn $(aws iam list-attached-role-policies --role-name flyio-chatlog-s3-access --query 'AttachedPolicies[0].PolicyArn' --output text) \
  --version-id v1
```

### Application shows "WARNING: Using static credentials"

**Cause:** AWS_ROLE_ARN not set or config.yaml has static credentials

**Solution:**
```bash
# Check Fly.io secrets
fly secrets list

# Ensure AWS_ROLE_ARN is set
fly secrets set AWS_ROLE_ARN="arn:aws:iam::123456789012:role/flyio-chatlog-s3-access"

# Redeploy
fly deploy
```

## Rollback to Static Credentials

If you need to rollback:

```bash
# Set old static credentials
fly secrets set S3_ACCESS_KEY_ID="your_access_key"
fly secrets set S3_SECRET_ACCESS_KEY="your_secret_key"

# Remove OIDC role
fly secrets unset AWS_ROLE_ARN

# Redeploy
fly deploy
```

The application will automatically fall back to static credential authentication.

## Security Best Practices

✅ **Implemented:**
- No long-lived credentials in Fly.io secrets
- Least privilege IAM policy (S3 only)
- Restricted IAM role trust policy (specific org/app)
- S3 bucket encryption enabled
- Public access blocked on S3 bucket
- HTTPS enforced via bucket policy

✅ **Recommended:**
- Enable AWS CloudTrail for audit logging
- Set up CloudWatch alarms for failed AssumeRole attempts
- Regularly review IAM policy permissions
- Enable S3 access logging for compliance

## Cost Impact

**Infrastructure Costs:**
- IAM OIDC Provider: **Free**
- IAM Role & Policies: **Free**
- S3 Storage: ~$0.023/GB/month (Standard), $0.004/GB/month (Glacier)
- S3 Requests: $0.005 per 1,000 PUT requests

**Lifecycle Savings:**
- After 90 days, logs move to Glacier (~83% cost reduction)
- Multipart upload cleanup prevents wasted storage
- Versioning cleanup prevents old version accumulation

## Next Steps

1. **Monitor CloudWatch Metrics:** Track S3 upload success rates
2. **Configure Alerts:** Set up alarms for upload failures
3. **Review Lifecycle Policies:** Adjust transition/expiration days if needed
4. **Enable S3 Inventory:** Track storage usage over time
5. **Document Procedures:** Update runbooks for team members

## Support

- Terraform issues: Check `terraform plan` output
- AWS IAM issues: Review CloudTrail logs
- Fly.io issues: Check `fly logs` and `fly status`
- Application issues: Review Go application logs

For Fly.io OIDC documentation: https://fly.io/docs/reference/oidc/
