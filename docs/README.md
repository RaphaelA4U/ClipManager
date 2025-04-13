# ClipManager 🎥

<p align="left">
  <img src="../static/img/ClipManager.png" alt="ClipManager Logo" width="400">
</p>

ClipManager is a simple, fast, and lightweight tool to record clips from an RTSP camera and send them to Telegram, Mattermost, Discord, or upload to SFTP.

## Features

- **Seamless Backtracking**: Capture moments that already happened, up to 5 minutes (300 seconds) in the past.
- **Configurable Duration**: Set custom lengths for your clips.
- **Multi-Platform Delivery**:
  - **Telegram**: Send clips to channels or chats
  - **Mattermost**: Post to your team's channels
  - **Discord**: Share via webhooks
  - **SFTP**: Upload to your server for storage
- **Clip Management**: Browse, play, download, and delete clips from the web interface.
- **Real-time Updates**: WebSocket notifications when new clips are created.
- **Integration Options**: Embed in your applications via API or custom buttons.
- **YouTube Upload**: Upload clips directly to your YouTube channel with customizable titles, descriptions, and privacy settings.

## Requirements

- Docker and Docker Compose.
- An RTSP camera (e.g., `rtsp://username:password@camera-ip:port/path`).
- Credentials for your chosen platform(s):
  - Telegram: Bot token and chat ID.
  - Mattermost: Server URL, API token, and channel ID.
  - Discord: Webhook URL.
  - SFTP: Host, port, username, password, and optional remote path.

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
2. Open `.env` in a text editor and add your camera's RTSP URL:
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

**Note**: If you're using the optional button integration, you'll need this server URL to configure the button script. See [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md) for details.

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
| `chat_app`          | string | Yes      | -       | Comma-separated list of platforms (`telegram`, `mattermost`, `discord`, `sftp`) |
| `category`          | string | No       | -       | Optional label to categorize clips              |
| `team1`             | string | No       | -       | Name of first team (for sports clips)           |
| `team2`             | string | No       | -       | Name of second team (for sports clips)          |
| `additional_text`   | string | No       | -       | Additional description text to append to clip message (not used for SFTP) |

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

#### SFTP
| Parameter           | Type   | Required | Default | Description                     |
|---------------------|--------|----------|--------|---------------------------------|
| `sftp_host`         | string | Yes      | -      | SFTP server hostname or IP      |
| `sftp_port`         | string | No       | 22     | SFTP server port                |
| `sftp_user`         | string | Yes      | -      | SFTP username                   |
| `sftp_password`     | string | Yes      | -      | SFTP password                   |
| `sftp_path`         | string | No       | .      | Remote path for file upload     |

### Response
Returns a JSON object with a `message` field indicating the request was received and processing has started.

### Notes
- SFTP filenames are dynamically generated based on optional parameters:
  - No optional parameters: `timestamp.mp4`
  - Only category: `category_timestamp.mp4`
  - Category, team1, team2: `category_team1_vs_team2_timestamp.mp4`
  - Only team1, team2: `team1_vs_team2_timestamp.mp4`
- SFTP uploads do not apply compression, unlike other chat apps.

## Troubleshooting
- **FFmpeg Errors**: Ensure `CAMERA_IP` is correct and the camera is accessible.
- **Chat Errors**: Verify your platform credentials (e.g., Mattermost token).
- **Server Not Accessible**: Check if Docker is running and the port (`HOST_PORT`) is not blocked by a firewall.
- **SFTP Connection Issues**: Verify hostname, port, credentials and that the server accepts password authentication.

For advanced usage, troubleshooting, and technical specifics, see [DEVELOPER.md](DEVELOPER.md).

## SFTP Management Features

ClipManager provides additional endpoints to manage clips stored on SFTP servers:

### SFTP Endpoints

#### `/api/clips` - List clips from the SFTP server
- **Method**: POST
- **Parameters**: Same SFTP parameters as above (`sftp_host`, `sftp_port`, `sftp_user`, `sftp_password`, `sftp_path`)
- **Response**: JSON array of clip information objects containing `name`, `size`, `mod_time`, and `path`

#### `/api/clips/test` - Test SFTP connection
- **Method**: POST
- **Parameters**: Same SFTP parameters as above
- **Response**: JSON object with `success` and `message` fields

#### `/api/clips/delete` - Delete a clip from the SFTP server
- **Method**: POST
- **Parameters**:
  - Same SFTP parameters as above
  - `path`: Path to the file to delete
- **Response**: JSON object with `success` and `message` fields

#### `/api/clip/stream` - Stream or download a clip from the SFTP server
- **Method**: GET
- **Query Parameters**:
  - Same SFTP parameters as above
  - `path`: Path to the file to stream
  - `download`: Set to `true` to download the file instead of streaming (optional)
- **Response**: Video file for direct playback in browser or download

### WebSocket Notifications

ClipManager supports real-time notifications for new clips uploaded to SFTP:

#### `/ws` - WebSocket endpoint for real-time notifications
- Connect to this WebSocket endpoint to receive notifications when new clips are uploaded
- Falls back to polling if WebSockets are not supported by the browser

## Optional Button Integration
Want to trigger clips with a physical button? ClipManager supports an optional button integration using an Arduino and a Windows PC. See [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md) for setup instructions.

Happy clipping! 🎥