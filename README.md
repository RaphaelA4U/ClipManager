# ClipManager

<p>
  <img src="img/ClipManager.png" alt="ClipManager Logo" width="400">
</p>

A simple, fast and lightweight application to record clips from an RTSP camera and send them to Telegram, Mattermost, or Discord.

## Requirements
- Docker and Docker Compose
- An RTSP camera (e.g. `rtsp://username:password@camera-ip:port/path`)
- One of the following:
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
