# ClipManager

A simple, fast and lightweight application to record clips from an RTSP camera and send them to Telegram or Mattermost.

## Requirements
- Docker and Docker Compose
- An RTSP camera (e.g. `rtsp://username:password@camera-ip:port/path`)
- A Telegram bot token and chat ID, or a Mattermost server with API token and channel ID

## Installation
1. **Clone the repository**:
   ```bash
   git clone https://github.com/RaphaelA4U/ClipManager
   cd clipmanager
   ```

2. **Configure the port (optional)**: Copy `.env.example` to `.env` and set the port (default 8080):
   ```bash
   cp .env.example .env
   ```
   
   Edit `.env` if needed:
   ```
   PORT=8080
   ```

3. **Start the application**:
   ```bash
   docker-compose up --build
   ```

4. **Check the logs**: At startup, you will see a message like:
   ```
   ClipManager started! Make a GET/POST request to localhost:8080/api/clip with parameters: camera_ip, chat_app, bot_token, chat_id, backtrack_seconds, duration_seconds
   ```

## Usage

Make a GET or POST request to `localhost:8080/api/clip` with the following parameters:

### Common Parameters
| Parameter | Description | Example | Required |
|-----------|-------------|-----------|-----------|
| camera_ip | The RTSP URL of the camera | rtsp://username:password@camera-ip:port/path | Yes |
| chat_app | The chat app ("telegram" or "mattermost") | telegram | Yes |
| backtrack_seconds | Number of seconds to go back for recording | 10 | Yes |
| duration_seconds | Duration of the clip in seconds | 10 | Yes |

### Telegram-specific Parameters
| Parameter | Description | Example | Required for Telegram |
|-----------|-------------|-----------|-----------|
| bot_token | The Telegram bot token | 123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ | Yes |
| chat_id | The Telegram chat ID | -100123456789 | Yes |

### Mattermost-specific Parameters
| Parameter | Description | Example | Required for Mattermost |
|-----------|-------------|-----------|-----------|
| mattermost_url | The URL of the Mattermost server | https://mattermost.example.com | Yes |
| mattermost_token | The Mattermost API token | abcdefghijklmnopqrstuvwxyz | Yes |
| mattermost_channel | The Mattermost channel ID | 123456789abcdefghijklmn | Yes |

### GET example (Telegram):
```bash
curl "localhost:8080/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&chat_app=telegram&bot_token=123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ&chat_id=-100123456789&backtrack_seconds=10&duration_seconds=10"
```

### GET example (Mattermost):
```bash
curl "localhost:8080/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&chat_app=mattermost&mattermost_url=https://mattermost.example.com&mattermost_token=abcdefghijklmnopqrstuvwxyz&mattermost_channel=123456789abcdefghijklmn&backtrack_seconds=10&duration_seconds=10"
```

### POST example (Telegram):
```bash
curl -X POST localhost:8080/api/clip -H "Content-Type: application/json" -d '{"camera_ip":"rtsp://username:password@camera-ip:port/path","chat_app":"telegram","bot_token":"123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ","chat_id":"-100123456789","backtrack_seconds":10,"duration_seconds":10}'
```

### POST example (Mattermost):
```bash
curl -X POST localhost:8080/api/clip -H "Content-Type: application/json" -d '{"camera_ip":"rtsp://username:password@camera-ip:port/path","chat_app":"mattermost","mattermost_url":"https://mattermost.example.com","mattermost_token":"abcdefghijklmnopqrstuvwxyz","mattermost_channel":"123456789abcdefghijklmn","backtrack_seconds":10,"duration_seconds":10}'
```

### Response

On success:
```json
{"message":"Clip recorded and sending started"}
```

On errors, you will receive an HTTP error code with a description.

## Notes

- The clip is stored locally in the `clips` directory and deleted after sending.
- No database is used; the app is completely stateless.
- The app is optimized for speed and uses a minimal Go binary with FFmpeg.
- For maximum performance, the clip is sent asynchronously to the messaging service.
- When compressed, the app preserves the original aspect ratio of the video.

## Troubleshooting

- **FFmpeg errors**: Make sure the `camera_ip` is correct and the RTSP stream is accessible.
- **Telegram errors**: Check if the `bot_token` and `chat_id` are correct.
- **Mattermost errors**: Check if the `mattermost_url`, `mattermost_token`, and `mattermost_channel` are correct.
- **Logs**: View the Docker logs for more information:
  ```bash
  docker-compose logs
  ```
