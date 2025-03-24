# ClipManager

<p>
  <img src="static/img/ClipManager.png" alt="ClipManager Logo" width="400">
</p>

A simple, fast and lightweight application to record clips from an RTSP camera and send them to Telegram, Mattermost, or Discord.

## Features

- Record clips from any RTSP camera
- Send clips to multiple messaging platforms simultaneously:
  - Telegram
  - Mattermost
  - Discord
- Categorize clips for better organization
- Automatic compression for large videos
- Web interface for easy configuration
- Integration with PoolManager for team and match information
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

2. **Start the application**:
   ```bash
   docker-compose up --build
   ```

3. **Access the application**:
   By default, the application will be available at `http://localhost:5001`

## Using the Web Interface

The ClipManager includes a user-friendly web interface accessible at the root URL (`http://localhost:5001/`).

### Configuration Tab

1. Enter your camera's RTSP URL in the "Camera IP" field
2. Set the desired "Backtrack Seconds" (how far back to start recording)
3. Set the "Duration Seconds" (length of the clip)
4. Select one or more messaging platforms (Telegram, Mattermost, Discord)
5. Enter platform-specific credentials for each selected platform
6. Optionally add a category to organize your clips
7. Enable PoolManager Connection if you want to include team and match information
8. Click "Save" to store your configuration
9. Click "Record Clip" to capture and send a clip with these settings

### Integration Tab

After saving your configuration, you can access integration options:

1. QR Code: Scan with a mobile device to trigger recording
2. HTML Button Code: Copy embed code for websites or dashboards
3. cURL Command: Copy command for terminal or script integration

## Docker Port Configuration

The ClipManager uses port 5000 inside the container, but you can map it to any port on your host machine. By default, it's mapped to port 5001.

### Understanding Port Mapping in Docker

In the `docker-compose.yml` file, the port mapping follows this format:
```
"HOST_PORT:CONTAINER_PORT"
```

For example, with `"5001:5000"`:
- `5000` - Internal container port (the app listens on this port inside Docker)
- `5001` - Host port (you'll access the app on this port from your browser)

This means you would access the application at `http://localhost:5001`.

### Changing the Host Port

To change the port that's accessible on your host machine, modify both:
1. The first number in the `ports` mapping
2. The `HOST_PORT` environment variable to match

```yml
services:
  clipmanager:
    # ...
    ports:
      - "8080:5000"  # Maps host port 8080 to container port 5000
    environment:
      - PORT=5000
      - HOST_PORT=8080  # Update this to match the first number in ports
```

With this configuration:
- The application will listen on port 5000 inside the container
- You'll access it from your host machine at `http://localhost:8080`
- The application logs will show the correct access URLs with port 8080

### Example Port Configurations

1. **Default configuration** - access on port 5001:
   ```yml
   ports:
     - "5001:5000"
   environment:
     - PORT=5000
     - HOST_PORT=5001
   ```
   Access the application at: `http://localhost:5001/api/clip`

2. **Alternative port 8080** - useful if port 5001 is already in use:
   ```yml
   ports:
     - "8080:5000"
   environment:
     - PORT=5000
     - HOST_PORT=8080
   ```
   Access the application at: `http://localhost:8080/api/clip`

3. **Using multiple instances** on different ports:
   ```yml
   # First instance in docker-compose.yml
   ports:
     - "8081:5000"
   environment:
     - PORT=5000
     - HOST_PORT=8081
   
   # Second instance in another docker-compose file
   ports:
     - "8082:5000"
   environment:
     - PORT=5000
     - HOST_PORT=8082
   ```

### Starting with a Custom Port

After changing the port in `docker-compose.yml`, restart the application:

```bash
# To stop the current instance first
docker-compose down

# To start with the new configuration
docker-compose up
```

## Usage

Make a GET or POST request to the application with the following parameters:

### Example Request URL

```
http://localhost:5001/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=telegram&telegram_bot_token=YOUR_BOT_TOKEN&telegram_chat_id=YOUR_CHAT_ID
```

Remember to replace the host port in the URL if you've changed it in your docker-compose.yml.

### Parameters (in logical order)

#### Common Parameters
| Parameter | Description | Example | Required |
|-----------|-------------|-----------|-----------|
| camera_ip | The RTSP URL of the camera | rtsp://username:password@camera-ip:port/path | Yes |
| backtrack_seconds | Number of seconds to go back for recording (5-300) | 10 | Yes |
| duration_seconds | Duration of the clip in seconds (5-300) | 10 | Yes |
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

#### poolmanager_connection
- **Description**: Enables integration with PoolManager to include team and match information in the clip metadata.
- **Example**: `poolmanager_connection=true`
- **Required**: No

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

- The clip is stored locally in the `clips` directory and deleted after sending.
- No database is used; the app is completely stateless.
- The app is optimized for speed and uses a minimal Go binary with FFmpeg.
- For maximum performance, the clip is sent asynchronously to the messaging service.
- When compressed, the app preserves the original aspect ratio of the video.
- Videos larger than 50MB are automatically compressed to reduce file size while maintaining quality.

## Troubleshooting

### Port Conflicts

If you see an error like `bind: address already in use` when starting the container, port 5000 is already being used by another application on your host machine. To solve this:

1. Change the host port in `docker-compose.yml`:
   ```yml
   ports:
     - "8080:5000"  # Use port 8080 instead of 5000
   ```

2. Restart the application:
   ```bash
   docker-compose down
   docker-compose up
   ```

3. Access the application using the new port:
   ```
   http://localhost:8080/api/clip
   ```

### Other Common Issues

- **FFmpeg errors**: Make sure the `camera_ip` is correct and the RTSP stream is accessible.
- **Telegram errors**: Check if the `telegram_bot_token` and `telegram_chat_id` are correct.
- **Mattermost errors**: Check if the `mattermost_url`, `mattermost_token`, and `mattermost_channel` are correct.
- **Discord errors**: Verify that the `discord_webhook_url` is valid and correctly formatted.
- **Logs**: View the Docker logs for more information:
  ```bash
  docker-compose logs
  ```
