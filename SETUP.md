# Development Setup

This guide will help you set up chatlog for local development and deployment.

## Prerequisites

- Go 1.23 or later
- Docker (for containerized deployment)
- fly.io CLI (for deployment)
- AWS CLI or S3-compatible client (for testing uploads)

## Local Development

### 1. Install Go

Download and install Go from https://go.dev/dl/

Verify installation:
```bash
go version
```

### 2. Clone and Setup

```bash
cd chatlog
go mod download
```

### 3. Configuration

Create your configuration file:
```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` with your values:

**Get Twitch OAuth Token**:
1. Visit https://twitchapps.com/tmi/
2. Authorize the application
3. Copy the OAuth token (starts with `oauth:`)
4. Add to config.yaml

**Setup S3**:
- For AWS S3: Create bucket and IAM user with S3 access
- For Cloudflare R2: Create bucket and API token
- Add credentials to config.yaml

### 4. Run Locally

```bash
go run .
```

Or build and run:
```bash
go build -o chatlog
./chatlog
```

### 5. Development Tips

**Using Environment Variables** (recommended for secrets):
```bash
export TWITCH_OAUTH="oauth:your_token"
export S3_ACCESS_KEY_ID="your_key"
export S3_SECRET_ACCESS_KEY="your_secret"
go run .
```

**Testing Without S3**:
Comment out S3 uploader initialization to test recording locally.

**File Output**:
Check `data/` directory for generated JSONL files.

## Deployment to fly.io

### 1. Install fly.io CLI

```bash
curl -L https://fly.io/install.sh | sh
```

### 2. Login

```bash
fly auth login
```

### 3. Create App

```bash
fly apps create chatlog
```

### 4. Create Volume

For persistent local file buffer:
```bash
fly volumes create chatlog_data --size 1 --region iad
```

### 5. Set Secrets

```bash
fly secrets set TWITCH_OAUTH="oauth:your_token"
fly secrets set S3_ACCESS_KEY_ID="your_key"
fly secrets set S3_SECRET_ACCESS_KEY="your_secret"
```

### 6. Deploy

```bash
fly deploy
```

### 7. Monitor

```bash
# View logs
fly logs

# Check status
fly status

# SSH into VM
fly ssh console
```

## Configuration Options

See `config.example.yaml` for all available configuration options.

**Key Settings**:
- `twitch.channels`: List of Twitch channels to monitor
- `recorder.rotate_minutes`: How often to rotate log files
- `recorder.buffer_size`: Message buffer size (affects memory usage)
- `uploader.delete_after_upload`: Remove local files after S3 upload

## S3 Storage Structure

Files are uploaded to S3 with the following structure:
```
s3://bucket-name/
  2025/
    12/
      29/
        twitch/
          shroud/
            twitch_shroud_20251229_1030.jsonl
            twitch_shroud_20251229_1130.jsonl
          summit1g/
            twitch_summit1g_20251229_1030.jsonl
```

## Troubleshooting

**Connection Issues**:
- Verify OAuth token is valid and starts with `oauth:`
- Check channel names don't include `#` prefix
- Ensure firewall allows outbound connections to irc.chat.twitch.tv:6697

**S3 Upload Failures**:
- Verify S3 credentials and bucket permissions
- Check bucket region matches configuration
- For R2/custom endpoints, verify endpoint URL is correct

**High Memory Usage**:
- Reduce `recorder.buffer_size`
- Decrease `recorder.rotate_minutes` to flush files more often
- Reduce number of channels being monitored

**fly.io Issues**:
- Check logs: `fly logs`
- Verify secrets are set: `fly secrets list`
- Ensure volume is mounted: `fly volumes list`

## Next Steps

1. Review [ARCHITECTURE.md](./ARCHITECTURE.md) for system design
2. Implement Twitch IRC connector
3. Implement message recorder
4. Implement S3 uploader
5. Add health check endpoint
6. Add monitoring/metrics

## Contributing

TBD

## License

TBD
