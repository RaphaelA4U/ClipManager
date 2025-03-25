# ClipManager

<p align="center">
  <img src="static/img/ClipManager.png" alt="ClipManager Logo" width="400">
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

## Example API Request

Record a 10-second clip from 10 seconds ago and send it to Telegram:
```bash
curl "http://localhost:5001/api/clip?backtrack_seconds=10&duration_seconds=10&chat_app=telegram&telegram_bot_token=YOUR_TOKEN&telegram_chat_id=YOUR_CHAT_ID"
```

## Need More Details?

For advanced usage, troubleshooting, and technical specifics, see [DEVELOPER.md](DEVELOPER.md).