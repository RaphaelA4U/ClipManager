# ClipManager

<p>
  <img src="static/img/ClipManager.png" alt="ClipManager Logo" width="400">
</p>

A simple, fast and lightweight application to record clips from an RTSP camera and send them to Telegram, Mattermost, or Discord.

## Features

- Record clips from any RTSP camera
- Real backtracking supported - record clips from up to 300 seconds in the past
- Automatic camera reconnection after disconnects
- Send clips to multiple messaging platforms simultaneously:
  - Telegram
  - Mattermost
  - Discord
- Categorize clips for better organization
- Automatic compression for large videos
- Web interface for easy configuration
<!-- - Integration with PoolManager for team and match information -->
- API endpoint for programmatic control
- Stateless design with no database requirements

## Requirements
- Docker and Docker Compose
- An RTSP camera (e.g. `rtsp://username:password@camera-ip:port/path`)
- One or more of the following:
  - A Telegram bot token and chat ID
  - A Mattermost server with API token and channel ID
  - A Discord webhook URL

## Quick Start

1. **Clone the repository**:
   ```bash
   git clone https://github.com/RaphaelA4U/ClipManager
   cd clipmanager
   ```

2. **Create environment configuration**:
   ```bash
   cp .env.example .env
   ```
   At minimum, edit the `.env` file to set your camera's RTSP URL:
   ```
   CAMERA_IP=rtsp://username:password@your-camera-ip:port/path
   ```
   The default ports (HOST_PORT=5001) will be used if not specified.

3. **Start the application**:
   ```bash
   docker-compose up --build
   ```

4. **Access the application**:
   Access the application at http://localhost:5001 (or the custom port if you specified a different HOST_PORT)

## Environment Configuration

ClipManager uses environment variables for configuration through the `.env` file:

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| CAMERA_IP | RTSP URL of your camera | Yes | None |
| HOST_PORT | External port to access the application | No | 5001 |

**Note**: If you only specify `CAMERA_IP` in your `.env` file, the application will use the default ports (HOST_PORT=5001).

## Using the Web Interface

The ClipManager includes a user-friendly web interface accessible at the root URL (`http://localhost:5001/`).

### Configuration Tab

1. The camera's RTSP URL is automatically set from your `.env` file
2. Set the desired "Backtrack Seconds" (how far back to start recording)
3. Set the "Duration Seconds" (length of the clip)
4. Select one or more messaging platforms (Telegram, Mattermost, Discord)
5. Enter platform-specific credentials for each selected platform
6. Optionally add a category to organize your clips
<!-- 7. Enable PoolManager Connection if you want to include team and match information -->
8. Click "Save" to store your configuration
9. Click "Record Clip" to capture and send a clip with these settings

### Integration Tab

After saving your configuration, you can access integration options:

1. QR Code: Scan with a mobile device to trigger recording
2. HTML Button Code: Copy embed code for websites or dashboards
3. cURL Command: Copy command for terminal or script integration

## Usage

Make a GET or POST request to the application with the following parameters:

### Example Request URL

```
http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=telegram&telegram_bot_token=YOUR_BOT_TOKEN&telegram_chat_id=YOUR_CHAT_ID
```

Remember to replace the host port in the URL if you've changed it in your .env file.

### Parameters (in logical order)

#### Common Parameters
| Parameter | Description | Example | Required |
|-----------|-------------|-----------|-----------|
| backtrack_seconds | Number of seconds to go back for recording (0-300) | 10 | Yes |
| duration_seconds | Duration of the clip in seconds (1-300) | 10 | Yes |
| chat_app | The chat app ("telegram", "mattermost", or "discord") | telegram | Yes |

#### Chat App-Specific Parameters

##### Telegram Parameters
| Parameter | Description | Example | Required for Telegram |
|-----------|-------------|-----------|-----------|
| telegram_bot_token | The Telegram bot token | 123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ | Yes |
| telegram_chat_id | The Telegram chat ID | -100123456789 | Yes |

##### Mattermost Parameters
| Parameter | Description | Example | Required for Mattermost |
|-----------|-------------|-----------|-----------|
| mattermost_url | The URL of the Mattermost server | https://mattermost.example.com | Yes |
| mattermost_token | The Mattermost API token | abcdefghijklmnopqrstuvwxyz | Yes |
| mattermost_channel | The Mattermost channel ID | 123456789abcdefghijklmn | Yes |

##### Discord Parameters
| Parameter | Description | Example | Required for Discord |
|-----------|-------------|-----------|-----------|
| discord_webhook_url | The Discord webhook URL | https://discord.com/api/webhooks/id/token | Yes |

### Optional Parameters

#### category
- **Description**: Categorizes clips for better organization. The category name will appear in the message sent to the selected platforms.
- **Example**: `category=match_highlights`
- **Required**: No
<!--
#### poolmanager_connection
- **Description**: Enables integration with PoolManager to include team and match information in the clip metadata.
- **Example**: `poolmanager_connection=true`
- **Required**: No
-->
## Using Multiple Chat Apps Simultaneously

ClipManager supports sending clips to multiple platforms at once. To do this:
1. Specify multiple platforms in the `chat_app` parameter, separated by commas (e.g., `telegram,mattermost,discord`).
2. Provide credentials for all selected platforms.

### Example Request for Multiple Platforms

**GET Request**:
```bash
curl "http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=telegram,mattermost,discord&telegram_bot_token=123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ&telegram_chat_id=-100123456789&mattermost_url=https://mattermost.example.com&mattermost_token=abcdefghijklmnopqrstuvwxyz&mattermost_channel=123456789abcdefghijklmn&discord_webhook_url=https://discord.com/api/webhooks/id/token"
```

**POST Request**:
```bash
curl -X POST http://localhost:5001/api/clip \
  -H "Content-Type: application/json" \
  -d '{
    "camera_ip": "rtsp://username:password@camera-ip:port/path",
    "backtrack_seconds": 10,
    "duration_seconds": 10,
    "chat_app": "telegram,mattermost,discord",
    "telegram_bot_token": "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ",
    "telegram_chat_id": "-100123456789",
    "mattermost_url": "https://mattermost.example.com",
    "mattermost_token": "abcdefghijklmnopqrstuvwxyz",
    "mattermost_channel": "123456789abcdefghijklmn",
    "discord_webhook_url": "https://discord.com/api/webhooks/id/token"
  }'
```

## Platform Setup

### Telegram
1. Create a bot using [BotFather](https://core.telegram.org/bots#botfather).
2. Save the bot token provided by BotFather.
3. Add the bot to a group or channel and promote it as an admin.
4. Use the [Telegram Bot API](https://core.telegram.org/bots/api#getupdates) or a tool like [getids](https://github.com/egor-tensin/getids) to find the chat ID.

### Mattermost
1. Log in to your Mattermost server.
2. Go to **Integrations > Bot Accounts** and create a bot account.
3. Generate an API token for the bot.
4. Find the channel ID by navigating to the channel and copying its URL. The channel ID is the last part of the URL.

### Discord
1. Go to your Discord server settings and create a new webhook under **Integrations > Webhooks**.
2. Copy the webhook URL.
3. Optionally, customize the webhook name and avatar.

### Example Requests

#### Telegram

**GET Request**:
```bash
curl "http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=telegram&telegram_bot_token=123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ&telegram_chat_id=-100123456789"
```

**POST Request**:
```bash
curl -X POST http://localhost:5001/api/clip \
  -H "Content-Type: application/json" \
  -d '{
    "camera_ip": "rtsp://username:password@camera-ip:port/path",
    "backtrack_seconds": 10,
    "duration_seconds": 10,
    "chat_app": "telegram",
    "telegram_bot_token": "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ",
    "telegram_chat_id": "-100123456789"
  }'
```

#### Mattermost

**GET Request**:
```bash
curl "http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=mattermost&mattermost_url=https://mattermost.example.com&mattermost_token=abcdefghijklmnopqrstuvwxyz&mattermost_channel=123456789abcdefghijklmn"
```

#### Discord

**GET Request**:
```bash
curl "http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=discord&discord_webhook_url=https://discord.com/api/webhooks/id/token"
```

### Updated Example Requests

### Telegram, Mattermost, and Discord Combined

**GET Request**:
```bash
curl "http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=telegram,mattermost,discord&telegram_bot_token=123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ&telegram_chat_id=-100123456789&mattermost_url=https://mattermost.example.com&mattermost_token=abcdefghijklmnopqrstuvwxyz&mattermost_channel=123456789abcdefghijklmn&discord_webhook_url=https://discord.com/api/webhooks/id/token"
```

**POST Request**:
```bash
curl -X POST http://localhost:5001/api/clip \
  -H "Content-Type: application/json" \
  -d '{
    "camera_ip": "rtsp://username:password@camera-ip:port/path",
    "backtrack_seconds": 10,
    "duration_seconds": 10,
    "chat_app": "telegram,mattermost,discord",
    "telegram_bot_token": "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ",
    "telegram_chat_id": "-100123456789",
    "mattermost_url": "https://mattermost.example.com",
    "mattermost_token": "abcdefghijklmnopqrstuvwxyz",
    "mattermost_channel": "123456789abcdefghijklmn",
    "discord_webhook_url": "https://discord.com/api/webhooks/id/token"
  }'
```

### Response

On success:
```json
{"message":"Clip recorded and sending started"}
```

On errors, you will receive an HTTP error status code with a descriptive error message.

## Notes

- Clips are recorded in 1920x1080 resolution for optimal quality, but may be compressed to 1280x720 if the file size exceeds 50MB
- The clip is stored locally in the `clips` directory and deleted after sending
- No database is used; the app is completely stateless
- The app is optimized for speed and uses a minimal Go binary with FFmpeg
- For maximum performance, the clip is sent asynchronously to the messaging service
- When compressed, the app preserves the original aspect ratio of the video
- Videos larger than 50MB are automatically compressed to reduce file size while maintaining quality

## Troubleshooting

### Common Issues

- **FFmpeg errors**: Make sure the `camera_ip` is correct and the RTSP stream is accessible
- **Telegram errors**: Check if the `telegram_bot_token` and `telegram_chat_id` are correct
- **Mattermost errors**: Check if the `mattermost_url`, `mattermost_token`, and `mattermost_channel` are correct
- **Discord errors**: Verify that the `discord_webhook_url` is valid and correctly formatted
- **Logs**: View the Docker logs for more information:
  ```bash
  docker-compose logs
  ```

### Low Disk Space

- If available disk space drops below 500MB, ClipManager will pause background recording
- The application will automatically retry after 30 seconds
- To resolve, free up disk space by removing unnecessary files
- Background recording will resume automatically once sufficient space is available
