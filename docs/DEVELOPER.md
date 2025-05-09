# ClipManager Developer Guide

This document provides technical details for developers working on or integrating with ClipManager.

## Overview

ClipManager is a Go application that records RTSP streams in 5-second segments, enabling backtracking up to 300 seconds. It processes requests asynchronously and sends clips to Telegram, Mattermost, Discord, or uploads to SFTP servers, with additional SFTP management capabilities.

## Architecture

- **Segment Recording**: Continuous background recording using FFmpeg into `.ts` segments.
- **Clip Extraction**: Concatenates segments into `.mp4` files with FFmpeg.
- **Chat Integration**: Sends clips via HTTP APIs with platform-specific compression.
- **SFTP Management**: Browse, stream, download, and delete clips from SFTP servers.
- **WebSocket Notifications**: Real-time notifications when new clips are uploaded.
- **Web Interface**: HTML form served at `/` with API calls to `/api/clip`.

## Configuration

Environment variables in `.env`:
| Variable   | Description                        | Default |
|------------|------------------------------------|---------|
| `CAMERA_IP`| RTSP URL of the camera             | None    |
| `HOST_PORT`| External port for access           | 5001    |
| `PORT`     | Internal port (container)          | 5000    |

## API Endpoint

- **URL**: `/api/clip`
- **Methods**: GET, POST
- **Parameters**:
  - `backtrack_seconds` (0-300): Seconds to go back.
  - `duration_seconds` (1-300): Clip length.
  - `chat_app`: Comma-separated list (e.g., `telegram,discord`).
  - Platform-specific: See the main [README.md](README.md) for platform-specific parameters.

### Example POST Request
```json
{
  "backtrack_seconds": 10,
  "duration_seconds": 10,
  "chat_app": "telegram",
  "telegram_bot_token": "YOUR_TOKEN",
  "telegram_chat_id": "YOUR_CHAT_ID"
}
```

## Segment Management

- Segments are stored in `clips/` as `segment_cycleN_NNN.ts`.
- Maximum 300 seconds are kept, older ones are deleted.
- Timestamps are used to align segments with requested times.

## Logging

Logs use ANSI colors and emoji indicators:
- ℹ️ (Blue): Info
- ✅ (Green): Success
- ⚠️ (Yellow): Warning
- ❌ (Red): Error
- 🔧 (Cyan): Debug

Example:
```
2025/03/25 10:00:00 ✅ Added segment: segment_cycle0_000.ts, total: 62 (up to 310 seconds)
```

## Troubleshooting

- **FFmpeg Errors**: Check `CAMERA_IP` and network access.
- **Chat Errors**: Verify credentials and IDs.
- **Disk Space**: Needs >500MB free, else recording pauses.
- **Backtracking**: Full 300s available after ~5 minutes of runtime.

## Development Notes

- **Dependencies**: Managed via `go.mod`.
- **Docker**: Built in two stages (Golang builder + FFmpeg).
- **Extending**: Add new chat apps by implementing `sendToX` methods.

## SFTP Management API

ClipManager provides several endpoints for managing clips on SFTP servers:

### List Clips
- **Endpoint**: `/api/clips`
- **Handler**: `HandleListClips`
- **Method**: POST
- **Implementation**:
  - Connects to SFTP server using provided credentials
  - Lists files in specified directory
  - Filters for .mp4 files only
  - Returns metadata (name, size, modification time, path)

### Test Connection
- **Endpoint**: `/api/clips/test`
- **Handler**: `HandleTestSFTPConnection`
- **Method**: POST
- **Implementation**:
  - Attempts to connect to SFTP server
  - Tries to read the specified directory
  - Returns success/failure status with descriptive message

### Delete Clip
- **Endpoint**: `/api/clips/delete`
- **Handler**: `HandleDeleteClip`
- **Method**: POST
- **Implementation**:
  - Connects to SFTP server
  - Attempts to remove specified file
  - Returns success/failure status with descriptive message

### Stream/Download Clip
- **Endpoint**: `/api/clip/stream`
- **Handler**: `HandleStreamClip`
- **Method**: GET
- **Implementation**:
  - Opens file from SFTP server
  - Sets appropriate content headers for streaming or downloading
  - Serves content directly to the client browser
  - Supports both inline viewing and file download

## WebSocket Implementation

ClipManager uses WebSockets to notify clients about newly uploaded clips:

- **Endpoint**: `/ws`
- **Handler**: `HandleWebSocket`
- **Implementation**:
  - Uses `github.com/gorilla/websocket` package
  - Maintains a map of connected clients with thread-safe access
  - Clients are notified via `broadcastNewClip` when uploads complete
  - Client-side implements fallback to polling when WebSockets are unavailable
  - Supports reconnection if connection is lost

### SFTP File Naming

The `generateSFTPFilename` function creates filenames based on request parameters:
- Uses title as the primary identifier (falls back to category if title is empty)
- Sanitizes inputs to remove invalid characters
- Formats with timestamp (YYYY-MM-DD_HH-MM)
- Creates meaningful formatted filenames based on available parameters:
  - With title only: `title_timestamp.mp4`
  - With title and category: `title_category_timestamp.mp4` 
  - With title, category, teams: `title_category_team1_vs_team2_timestamp.mp4`

The title field is optional, and if not provided, the function falls back to using category as the title.

## Optional Button Integration
ClipManager supports an optional button integration using an Arduino and a PowerShell script on a Windows PC to trigger clip recordings with a physical button. For details on implementation and setup, see [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md).

## YouTube Upload Integration

ClipManager supports direct uploads to YouTube via the YouTube Data API v3. This feature allows users to upload clips to their personal YouTube channels.

### Authentication Flow

- Uses Google's JavaScript client (`gapi`) to manage OAuth2 authentication
- Authenticates users with popup consent window
- Requests scope `https://www.googleapis.com/auth/youtube.upload`
- Stores access tokens in browser's localStorage
- No server-side token storage - authentication is entirely client-side

### Upload Process

- Each clip in the UI has an "Upload" button
- Clicking the button checks if the user is authenticated with YouTube
- If not authenticated, prompts for YouTube login
- Once authenticated, displays a modal with fields for:
  - Video title (defaults to clip name)
  - Description
  - Privacy setting (Public/Unlisted/Private)
- Files are uploaded directly from the browser to YouTube using multipart upload
- Progress is shown during upload
- User receives success or error notifications

### Implementation Details

- Uses the YouTube Data API v3 endpoint: `https://www.googleapis.com/upload/youtube/v3/videos`
- Files are streamed from SFTP to browser, then to YouTube
- Upload uses a two-step process:
  1. Initialize upload session with metadata
  2. Upload video data to the provided upload URL
- Error handling includes YouTube quota limits, authentication issues, and network problems

### User Account Management

- Users can connect their own YouTube account
- Users can disconnect accounts to switch to different ones
- Authentication state is preserved between sessions

See the source code for detailed implementation.