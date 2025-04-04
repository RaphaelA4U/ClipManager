# ClipManager 🎥

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
- **Optional Button Integration**: Trigger clips with a physical button using an Arduino (see [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md) for details).

## Requirements

- Docker and Docker Compose.
- An RTSP camera (e.g., `rtsp://username:password@camera-ip:port/path`).
- Credentials for your chosen platform(s):
  - Telegram: Bot token and chat ID.
  - Mattermost: Server URL, API token, and channel ID.
  - Discord: Webhook URL.

## Quick Start

### Step 1: Clone the Repository
1. Clone the repository to your server:
   ```bash
   git clone https://github.com/RaphaelA4U/ClipManager
   cd ClipManager
   ```

### Step 2: Set Up the Environment
1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```
2. Open `.env` in a text editor and add your camera’s RTSP URL:
   ```
   CAMERA_IP=rtsp://username:password@your-camera-ip:port/path
   ```
   - (Optional) Adjust `HOST_PORT` (default: `5001`) if needed:
     ```
     HOST_PORT=5001
     ```

### Step 3: Run the App
1. Start the server with Docker Compose:
   ```bash
   docker-compose up --build -d
   ```
2. The server will run on `http://<server-ip>:5001` (or `http://localhost:5001` if running locally).
   - Replace `<server-ip>` with the IP address of the machine running the server.
   - If you have a domain (e.g., `clip.your-server.com`), use that instead.

### Step 4: Access the Web Interface
1. Open your browser and go to `http://<server-ip>:5001`.
2. Configure your clip settings (backtrack, duration, chat apps, etc.).
3. Save your settings and click "Record Clip" to capture and send.

**Note**: If you’re using the optional button integration, you’ll need this server URL to configure the button script. See [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md) for details.

## API Documentation

### Endpoint: `/api/clip`

An endpoint for recording and sending video clips from an RTSP camera stream.

### Methods Supported
- `GET` - Request a clip via URL parameters
- `POST` - Request a clip via JSON body

### Parameters
| Parameter           | Type   | Required | Default | Description                                      |
|---------------------|--------|----------|---------|--------------------------------------------------|
| `camera_ip`         | string | Yes*     | From `.env` | RTSP URL for the camera                      |
| `backtrack_seconds` | int    | No       | 0       | Seconds to rewind before recording (0-300)      |
| `duration_seconds`  | int    | Yes      | -       | Length of clip to record in seconds (1-300)     |
| `chat_app`          | string | Yes      | -       | Comma-separated list of platforms (`telegram`, `mattermost`, `discord`) |
| `category`          | string | No       | -       | Optional label to categorize clips              |
| `team1`             | string | No       | -       | Name of first team (for sports clips)           |
| `team2`             | string | No       | -       | Name of second team (for sports clips)          |
| `additional_text`   | string | No       | -       | Additional description text to append to clip message |

*Required if not specified in the `.env` file.

### Platform-Specific Parameters

#### Telegram
| Parameter           | Type   | Required | Description                     |
|---------------------|--------|----------|---------------------------------|
| `telegram_bot_token`| string | Yes      | Telegram Bot API token          |
| `telegram_chat_id`  | string | Yes      | Target chat/channel ID          |

#### Mattermost
| Parameter           | Type   | Required | Description                     |
|---------------------|--------|----------|---------------------------------|
| `mattermost_url`    | string | Yes      | Mattermost server URL (no trailing slash) |
| `mattermost_token`  | string | Yes      | User or bot access token        |
| `mattermost_channel`| string | Yes      | Target channel ID               |

#### Discord
| Parameter           | Type   | Required | Description                     |
|---------------------|--------|----------|---------------------------------|
| `discord_webhook_url`| string | Yes      | Discord webhook URL             |

### Response
Returns a JSON object with a `message` field indicating the request was received and processing has started.

## Troubleshooting
- **FFmpeg Errors**: Ensure `CAMERA_IP` is correct and the camera is accessible.
- **Chat Errors**: Verify your platform credentials (e.g., Mattermost token).
- **Server Not Accessible**: Check if Docker is running and the port (`HOST_PORT`) is not blocked by a firewall.

For advanced usage, troubleshooting, and technical specifics, see [DEVELOPER.md](DEVELOPER.md).

## Optional Button Integration
Want to trigger clips with a physical button? ClipManager supports an optional button integration using an Arduino and a Windows PC. See [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md) for setup instructions.

Happy clipping! 🎥