# Fly.io OIDC Implementation Fixes

This document outlines the fixes applied to correctly implement Fly.io OIDC authentication with AWS.

## Issues Fixed

### 1. ❌ Incorrect OIDC Issuer URL
**Before:** `https://oidc.fly.io/v1/`
**After:** `https://oidc.fly.io/<org-slug>` (organization-specific)

### 2. ❌ Wrong Audience Configuration
**Before:** Used Fly.io org name as audience
**After:** `sts.amazonaws.com` (AWS STS requirement)

### 3. ❌ Incorrect Subject Format
**Before:** `app:chatlog`
**After:** `<org>:<app>:*` (e.g., `personal:chatlog:*`)

### 4. ❌ Non-existent Token File Path
**Before:** Tried to read from `/var/run/fly/token` (doesn't exist)
**After:** Fetch tokens via Fly.io Unix socket API at `/.fly/api`

---

## Changes Made

### Terraform Configuration

#### `terraform/oidc.tf`

**OIDC Provider:**
```hcl
# Before
url = "https://oidc.fly.io/v1/"
client_id_list = [var.flyio_org]  # Wrong

# After
url = "https://oidc.fly.io/${var.flyio_org}"
client_id_list = ["sts.amazonaws.com"]  # Correct
```

**Trust Policy:**
```hcl
# Before
Condition = {
  StringEquals = {
    "oidc.fly.io/v1/:aud" = var.flyio_org
    "oidc.fly.io/v1/:sub" = "app:${var.flyio_app_name}"
  }
}

# After
Condition = {
  StringEquals = {
    "oidc.fly.io/${var.flyio_org}:aud" = "sts.amazonaws.com"
  }
  StringLike = {
    "oidc.fly.io/${var.flyio_org}:sub" = "${var.flyio_org}:${var.flyio_app_name}:*"
  }
}
```

### Go Application

#### `internal/uploader/uploader.go`

**Added Unix Socket Token Retriever:**
```go
type flyTokenRetriever struct {
    socketPath string
    audience   string
}

func (f *flyTokenRetriever) GetIdentityToken() ([]byte, error) {
    // Create HTTP client with Unix socket transport
    client := &http.Client{
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
                return net.Dial("unix", f.socketPath)
            },
        },
        Timeout: 5 * time.Second,
    }

    // POST to Fly.io OIDC API
    reqBody, _ := json.Marshal(map[string]string{
        "aud": f.audience,  // "sts.amazonaws.com"
    })

    resp, err := client.Post(
        "http://localhost/v1/tokens/oidc",
        "application/json",
        bytes.NewReader(reqBody),
    )

    // ... handle response and return token
}
```

**Updated Token Provider:**
```go
// Before
credProvider := stscreds.NewWebIdentityRoleProvider(
    stsClient,
    roleARN,
    stscreds.IdentityTokenFile("/var/run/fly/token"),  // Doesn't exist!
)

// After
tokenRetriever := &flyTokenRetriever{
    socketPath: "/.fly/api",
    audience:   "sts.amazonaws.com",
}
credProvider := stscreds.NewWebIdentityRoleProvider(
    stsClient,
    roleARN,
    tokenRetriever,
)
```

---

## How It Works Now

### Token Acquisition Flow

1. **Application starts** and creates S3 uploader with OIDC
2. **AWS SDK needs credentials** for S3 operations
3. **Token retriever called** via `GetIdentityToken()`
4. **HTTP POST** to Unix socket `/.fly/api/v1/tokens/oidc` with `{"aud":"sts.amazonaws.com"}`
5. **Fly.io returns JWT** with 15-minute expiration
6. **AWS SDK assumes role** using `AssumeRoleWithWebIdentity`
7. **Temporary credentials** obtained and cached
8. **S3 operations** proceed with temporary credentials

### OIDC Token Claims

The JWT token contains:
```json
{
  "iss": "https://oidc.fly.io/personal",
  "aud": "sts.amazonaws.com",
  "sub": "personal:chatlog:machine-id",
  "exp": 1735000000,
  ...
}
```

### AWS Trust Policy Validation

AWS validates:
1. ✅ Issuer matches OIDC provider URL
2. ✅ Audience is `sts.amazonaws.com`
3. ✅ Subject matches pattern `personal:chatlog:*`

---

## Deployment Checklist

Before deploying, ensure:

- [ ] Terraform updated with new OIDC configuration
- [ ] Run `terraform apply` to update IAM resources
- [ ] Application rebuilt with Unix socket token retriever
- [ ] `AWS_ROLE_ARN` secret set in Fly.io
- [ ] Deploy to Fly.io with `fly deploy`
- [ ] Verify OIDC API accessible: `curl --unix-socket /.fly/api ...`
- [ ] Check logs for successful S3 uploads
- [ ] Remove old static credentials from Fly.io secrets

---

## Testing OIDC Locally

You cannot test Fly.io OIDC locally because:
- The `/.fly/api` Unix socket only exists on Fly Machines
- Local development should use static credentials (legacy mode)

The application automatically falls back to static credentials if `AWS_ROLE_ARN` is not set.

---

## Reference Documentation

- [Fly.io OIDC Documentation](https://fly.io/docs/security/openid-connect/)
- [AWS OIDC Identity Providers](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
- [AWS SDK for Go v2 - Web Identity](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/credentials/stscreds)

---

## Summary

The key insight is that Fly.io OIDC tokens are **not stored in files** but are **dynamically fetched** via the Fly Machine's Unix socket API. The implementation now correctly:

1. ✅ Uses organization-specific OIDC issuer URL
2. ✅ Requests tokens with AWS audience (`sts.amazonaws.com`)
3. ✅ Fetches tokens from Unix socket API (`/.fly/api`)
4. ✅ Validates subject with org:app:machine pattern
5. ✅ Handles token expiration and refresh automatically (AWS SDK)

The application is now ready for deployment with proper OIDC authentication! 🚀
