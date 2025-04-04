# ClipManager Developer Guide

This document provides technical details for developers working on or integrating with ClipManager.

## Overview

ClipManager is a Go application that records RTSP streams in 5-second segments, enabling backtracking up to 300 seconds. It processes requests asynchronously and sends clips to Telegram, Mattermost, or Discord.

## Architecture

- **Segment Recording**: Continuous background recording using FFmpeg into `.ts` segments.
- **Clip Extraction**: Concatenates segments into `.mp4` files with FFmpeg.
- **Chat Integration**: Sends clips via HTTP APIs with platform-specific compression.
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

## Optional Button Integration
ClipManager supports an optional button integration using an Arduino and a PowerShell script on a Windows PC to trigger clip recordings with a physical button. For details on implementation and setup, see [ARDUINO_BUTTON.md](ARDUINO_BUTTON.md).

See the source code for detailed implementation.