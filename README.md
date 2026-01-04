# Chatlog

A lightweight, efficient service for recording livestream chats from platforms like Twitch and Kick.

## Overview

Chatlog is designed to run 24/7 to capture chat messages from livestreams, which can start unpredictably at any time. The service focuses on reliable, low-cost operation by efficiently recording chats and persisting them to S3 storage.

## Goals

- **24/7 Operation**: Run continuously to never miss chat messages from streams
- **Low Cost**: Minimal resource usage optimized for platforms like fly.io
- **Multi-Platform**: Support Twitch (priority), Kick, and potentially other platforms
- **Reliable Storage**: Persist chat logs to S3 for long-term, cost-effective storage
- **Simple Architecture**: File-based recording with focus on reliability over complex features

## Supported Platforms

- [x] Twitch (initial priority)
- [ ] Kick (planned)

## Architecture

Chatlog uses a simple, efficient architecture:

1. **Chat Connectors**: Connect to platform chat APIs (IRC for Twitch, WebSocket for Kick)
2. **Message Recorder**: Buffer and write messages to local files
3. **S3 Uploader**: Periodically upload completed chat logs to S3
4. **Configuration**: YAML-based config for channels to monitor

## Tech Stack

- **Language**: Go (efficient, great concurrency support)
- **Deployment**: fly.io (low-cost VMs with quick start times)
- **Storage**: S3-compatible storage (AWS S3, Cloudflare R2, etc.)
- **Format**: JSON Lines for easy processing and streaming

## Quick Start

See [SETUP.md](./SETUP.md) for development setup instructions.

## Project Status

🚧 **Early Development** - Initial project structure and documentation phase

## License

TBD
