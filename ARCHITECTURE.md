# Architecture

## Overview

Chatlog is designed as a simple, reliable service that runs 24/7 to capture livestream chats. The architecture prioritizes efficiency, low resource usage, and fault tolerance.

## Design Principles

1. **Keep It Simple**: Avoid over-engineering; focus on core functionality
2. **Fail Gracefully**: Network issues are expected; reconnect automatically
3. **Optimize for Cost**: Minimize memory and CPU usage for low fly.io costs
4. **Fire and Forget**: Once uploaded to S3, local data can be deleted
5. **Horizontal Concerns**: Each component has a single responsibility

## Components

### 1. Chat Connectors

Platform-specific connectors that establish and maintain connections to chat services.

**Twitch Connector** (`internal/twitch/`)
- Uses IRC protocol (irc.chat.twitch.tv:6697)
- Maintains persistent connection with automatic reconnection
- Parses IRC messages into structured format
- Handles Twitch-specific tags (badges, user IDs, etc.)

**Kick Connector** (`internal/kick/`) - Planned
- WebSocket-based connection
- Similar message parsing and reliability features

**Interface**: Each connector sends messages to a shared channel for recording.

### 2. Message Recorder

Central component that buffers and writes messages to disk (`internal/recorder/`).

**Responsibilities**:
- Receive messages from all platform connectors
- Buffer messages in memory (configurable size/time)
- Write to JSONL (JSON Lines) files for easy processing
- Rotate files based on size or time (e.g., hourly files)
- Signal uploader when files are complete and ready

**File Format**: JSONL (one JSON object per line)
```json
{"timestamp":"2025-12-29T10:30:45Z","channel":"shroud","username":"viewer123","user_id":"12345","message":"hello world"}
{"timestamp":"2025-12-29T10:30:47Z","channel":"shroud","username":"viewer456","user_id":"67890","message":"gg"}
```

**File Naming**: `{platform}_{channel}_{timestamp}.jsonl`
Example: `twitch_shroud_20251229_1030.jsonl`

### 3. S3 Uploader

Handles uploading completed log files to S3-compatible storage (`internal/uploader/`).

**Responsibilities**:
- Monitor for completed log files
- Upload to S3 with retry logic
- Verify upload success
- Delete local files after successful upload
- Support S3-compatible services (AWS S3, Cloudflare R2, etc.)

**S3 Key Structure**: `{year}/{month}/{day}/{platform}/{channel}/{filename}`
Example: `2025/12/29/twitch/shroud/twitch_shroud_20251229_1030.jsonl`

### 4. Configuration

YAML-based configuration (`internal/config/`).

**Example**:
```yaml
twitch:
  username: chatlog_bot
  oauth: oauth:your_token_here
  channels:
    - shroud
    - summit1g
    - xqc

s3:
  bucket: chatlog-archive
  region: us-east-1
  # Optional: for S3-compatible services
  endpoint: https://s3.amazonaws.com
  access_key_id: YOUR_KEY
  secret_access_key: YOUR_SECRET
```

## Data Flow

```
[Twitch IRC] ──> [Twitch Connector] ──┐
                                       ├──> [Message Channel] ──> [Recorder] ──> [Local JSONL Files] ──> [S3 Uploader] ──> [S3]
[Kick WebSocket] ──> [Kick Connector] ─┘
```

1. Connectors establish platform connections and parse messages
2. Messages flow through a shared Go channel to the recorder
3. Recorder buffers and writes to local JSONL files
4. Files are rotated based on time/size
5. Uploader picks up completed files and uploads to S3
6. Local files are deleted after successful upload

## Concurrency Model

- Each connector runs in its own goroutine
- Recorder runs in a dedicated goroutine
- Uploader runs in a dedicated goroutine
- All components listen to a shared context for graceful shutdown
- Channels are used for message passing between components

## Resource Optimization

**For fly.io low-cost operation**:

1. **Memory**: Keep buffer sizes small; flush to disk frequently
2. **CPU**: Minimal processing; simple JSON marshaling
3. **Network**: Persistent connections; avoid reconnection overhead
4. **Disk**: Delete local files after S3 upload; keep minimal local storage
5. **Graceful Shutdown**: Flush buffers and close connections properly on SIGTERM

## Error Handling

1. **Network Failures**: Automatic reconnection with exponential backoff
2. **Disk Errors**: Log and alert; retry or skip file
3. **S3 Upload Failures**: Retry with backoff; keep local file until success
4. **Parse Errors**: Log and skip malformed messages; don't crash

## Future Considerations

- Health check endpoint for fly.io health monitoring
- Metrics/monitoring (message rates, upload success, connection status)
- Multiple instances with channel sharding (if needed)
- Compression for S3 uploads to reduce storage costs
- Archive old S3 files to cheaper storage tiers
