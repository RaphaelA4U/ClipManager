# ClipManager

<p align="left">
  <img src="../static/img/ClipManager.png" alt="ClipManager Logo" width="400">
</p>

ClipManager is a simple, fast, and lightweight tool to record clips from an RTSP camera and send them to Telegram, Mattermost, or Discord.

## Features

- Record clips from any RTSP camera with up to 300 seconds of backtracking.
- Send clips to Telegram, Mattermost, or Discord (or all at once!).
- Add categories to organize your clips.
- Easy-to-use web interface for configuration.
- Automatic compression for large videos.
- API for programmatic control.

## Requirements

- Docker and Docker Compose.
- An RTSP camera (e.g., `rtsp://username:password@camera-ip:port/path`).
- Credentials for your chosen platform(s):
  - Telegram: Bot token and chat ID.
  - Mattermost: Server URL, API token, and channel ID.
  - Discord: Webhook URL.

## Quick Start

1. **Clone the repository**:
   ```bash
   git clone https://github.com/RaphaelA4U/ClipManager
   cd ClipManager
   ```

2. **Set up environment**:
   Copy `.env.example` to `.env` and edit it:
   ```bash
   cp .env.example .env
   ```
   Add your camera’s RTSP URL:
   ```
   CAMERA_IP=rtsp://username:password@your-camera-ip:port/path
   ```

3. **Run the app**:
   ```bash
   docker-compose up --build
   ```

4. **Access it**:
   Open `http://localhost:5001` in your browser.

## Using the Web Interface

1. Visit `http://localhost:5001`.
2. Configure your clip settings (backtrack, duration, chat apps, etc.).
3. Save your settings and click "Record Clip" to capture and send.

## API Documentation

### Endpoint: `/api/clip`

An endpoint for recording and sending video clips from an RTSP camera stream.

### Methods Supported

- `GET` - Request a clip via URL parameters
- `POST` - Request a clip via JSON body

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `camera_ip` | string | Yes* | From `.env` | RTSP URL for the camera |
| `backtrack_seconds` | int | No | 0 | Seconds to rewind before recording (0-300) |
| `duration_seconds` | int | Yes | - | Length of clip to record in seconds (1-300) |
| `chat_app` | string | Yes | - | Comma-separated list of platforms to send clip to (`telegram`, `mattermost`, `discord`) |
| `category` | string | No | - | Optional label to categorize clips |
| `team1` | string | No | - | Name of first team (for sports clips) |
| `team2` | string | No | - | Name of second team (for sports clips) |
| `additional_text` | string | No | - | Additional description text to append to clip message |

*Required if not specified in `.env` file

### Platform-Specific Parameters

#### Telegram
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `telegram_bot_token` | string | Yes | Telegram Bot API token |
| `telegram_chat_id` | string | Yes | Target chat/channel ID |

#### Mattermost
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `mattermost_url` | string | Yes | Mattermost server URL (no trailing slash) |
| `mattermost_token` | string | Yes | User or bot access token |
| `mattermost_channel` | string | Yes | Target channel ID |

#### Discord
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `discord_webhook_url` | string | Yes | Discord webhook URL |

### Response

Returns a JSON object with a `message` field indicating the request was received and processing has started.

For advanced usage, troubleshooting, and technical specifics, see [DEVELOPER.md](DEVELOPER.md).