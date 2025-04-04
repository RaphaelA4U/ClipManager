# ClipManager Button Integration (Optional)

This document explains how to set up the optional button integration for ClipManager. This feature allows you to trigger clip recordings by pressing a physical button connected to an Arduino, which communicates with a Windows PC or tablet to send requests to the ClipManager server.

---

## Overview

The button integration uses an Arduino with a push button to trigger clip recordings. When the button is pressed, a PowerShell script running on a Windows PC sends a request to the ClipManager server to record and send a clip to a chat platform (e.g., Mattermost, Telegram, or Discord). The category of the clip (e.g., "Hype" or "Blunder") is determined by the Arduino's identifier.

### What You Need
- A Windows PC or tablet (Windows 10 or higher).
- An Arduino (e.g., Arduino Uno) with a push button connected to pin 12.
- A USB cable to connect the Arduino to your computer.
- An internet connection (to communicate with the ClipManager server).
- The ClipManager server already running (see `README.md` for setup instructions).
- Credentials for your chosen chat platform (e.g., Mattermost token, Telegram bot token, or Discord webhook URL).

---

## Setup Instructions

### Step 1: Download the Button Integration Files
The button integration files are located in the `arduino_button/` directory of the ClipManager repository.

1. Go to the GitHub page of this project: [github.com/RaphaelA4U/ClipManager](https://github.com/RaphaelA4U/ClipManager).
2. Click the green **Code** button and select **Download ZIP**.
3. Extract the ZIP to a folder on your computer, e.g., `C:\Users\YourName\ClipManager`.
4. Navigate to the `arduino_button/` folder. You should see:
   ```
   arduino_button/
   ├── ClipManagerButton.ino
   ├── clipmanager.ps1
   ├── clipmanager_run.bat
   ├── config.json
   └── shortcut_clipmanager_run.bat
   ```

### Step 2: Configure the Server and Chat Settings
The PowerShell script (`clipmanager.ps1`) needs to know where the ClipManager server is running and how to send clips to your chat platform. This is configured in the `config.json` file.

1. Open `arduino_button/config.json` in a text editor (e.g., Notepad). You’ll see:
   ```json
   {
     "ServerUrl": "http://localhost:5001",
     "ChatApp": "",
     "ChatAppConfig": {
       "mattermost_url": "",
       "mattermost_channel": "",
       "mattermost_token": "",
       "telegram_bot_token": "",
       "telegram_chat_id": "",
       "discord_webhook_url": ""
     },
     "BacktrackSeconds": 60,
     "DurationSeconds": 60,
     "Team1": "",
     "Team2": "",
     "AdditionalText": ""
   }
   ```
2. Update the following fields:
   - **ServerUrl**: Replace `http://localhost:5001` with the URL of your ClipManager server (noted during the server setup in `README.md`). Examples:
     - If the server is running locally: `"ServerUrl": "http://localhost:5001"`
     - If the server is on another machine: `"ServerUrl": "http://192.168.1.100:5001"`
     - If you’re using a domain: `"ServerUrl": "http://clip.your-server.com"`
   - **ChatApp**: Specify the chat platform to send clips to. Valid options are:
     - `"mattermost"`
     - `"telegram"`
     - `"discord"`
   - **ChatAppConfig**: Provide the credentials for your chosen chat platform:
     - For Mattermost:
       - `"mattermost_url"`: Your Mattermost server URL (e.g., `"https://mm.your-server.com"`).
       - `"mattermost_channel"`: The channel ID (e.g., `"your-channel-id"`).
       - `"mattermost_token"`: Your Mattermost bot or user token (e.g., `"your-token"`).
     - For Telegram:
       - `"telegram_bot_token"`: Your Telegram bot token (e.g., `"your-bot-token"`).
       - `"telegram_chat_id"`: The chat ID (e.g., `"your-chat-id"`).
     - For Discord:
       - `"discord_webhook_url"`: Your Discord webhook URL (e.g., `"your-webhook-url"`).
     - Leave the unused fields empty (`""`).
   - **BacktrackSeconds**: The number of seconds to go back in the recording (0-300). Default is `60`.
   - **DurationSeconds**: The length of the clip in seconds (1-300). Default is `60`.
   - **Team1** (optional): The name of the first team, useful for sports clips (e.g., `"Team A"`). Leave empty (`""`) if not needed.
   - **Team2** (optional): The name of the second team (e.g., `"Team B"`). Leave empty (`""`) if not needed.
   - **AdditionalText** (optional): Additional description text to append to the clip message (e.g., `"Great moment in the game!"`). Leave empty (`""`) if not needed.

   **Example for Mattermost with Teams and Additional Text**:
   ```json
   {
     "ServerUrl": "http://192.168.1.100:5001",
     "ChatApp": " mattermost",
     "ChatAppConfig": {
       "mattermost_url": "https://mm.your-server.com",
       "mattermost_channel": "your-channel-id",
       "mattermost_token": "your-token",
       "telegram_bot_token": "",
       "telegram_chat_id": "",
       "discord_webhook_url": ""
     },
     "BacktrackSeconds": 60,
     "DurationSeconds": 60,
     "Team1": "Team A",
     "Team2": "Team B",
     "AdditionalText": "Great moment in the game!"
   }
   ```

   **Example for Telegram without Teams**:
   ```json
   {
     "ServerUrl": "http://192.168.1.100:5001",
     "ChatApp": "telegram",
     "ChatAppConfig": {
       "mattermost_url": "",
       "mattermost_channel": "",
       "mattermost_token": "",
       "telegram_bot_token": "your-bot-token",
       "telegram_chat_id": "your-chat-id",
       "discord_webhook_url": ""
     },
     "BacktrackSeconds": 30,
     "DurationSeconds": 30,
     "Team1": "",
     "Team2": "",
     "AdditionalText": "Check out this clip!"
   }
   ```

   **Example for Discord without Additional Text**:
   ```json
   {
     "ServerUrl": "http://192.168.1.100:5001",
     "ChatApp": "discord",
     "ChatAppConfig": {
       "mattermost_url": "",
       "mattermost_channel": "",
       "mattermost_token": "",
       "telegram_bot_token": "",
       "telegram_chat_id": "",
       "discord_webhook_url": "your-webhook-url"
     },
     "BacktrackSeconds": 45,
     "DurationSeconds": 45,
     "Team1": "Red Team",
     "Team2": "Blue Team",
     "AdditionalText": ""
   }
   ```

3. Save the file.

**Important**: If any required fields are missing or incorrect, the script will display a clear error message with instructions on how to fix it. The `Team1`, `Team2`, and `AdditionalText` fields are optional and can be left empty if not needed.

### Step 3: Configure the Arduino
The Arduino must be programmed to communicate with the PowerShell script. Follow these steps to upload the code:

#### 3.1 Download the Arduino Code
1. Open the file `arduino_button/ClipManagerButton.ino` in the Arduino IDE.
   - Double-click `ClipManagerButton.ino` to open it, or in the Arduino IDE, go to `File > Open` and select the file.

#### 3.2 Set the Identifier
- In the code, you’ll see a line:
  ```cpp
  const char* IDENTIFIER = "CLIPMANAGER_TEST";
  ```
- Change `"CLIPMANAGER_TEST"` to the desired category:
  - Use `"CLIPMANAGER_HYPE"` for the category "Hype".
  - Use `"CLIPMANAGER_BLUNDER"` for the category "Blunder".
  - Or choose a custom category, e.g., `"CLIPMANAGER_TEST"` for the category "Test".
- The category (everything after `CLIPMANAGER_`) will be used in the chat platform.

#### 3.3 Upload the Code to the Arduino
1. Connect your Arduino to your computer via the USB cable.
2. Open the Arduino IDE.
3. Select your Arduino in the IDE:
   - Go to `Tools > Board` and choose your Arduino (e.g., "Arduino Uno").
   - Go to `Tools > Port` and select the port your Arduino is connected to (e.g., COM3).
4. Click the **Upload** button (the right arrow) to upload the code to your Arduino.
5. Wait for the upload to complete. You should see a message like "Done uploading".

#### 3.4 (Optional) Test the Arduino
- Open the Serial Monitor in the Arduino IDE (`Tools > Serial Monitor`).
- Set the baud rate to `9600`.
- You should see a message like:
  ```
  Arduino gestart met identifier: CLIPMANAGER_HYPE
  ```
- Press the button; you should see "BUTTON_PRESSED" in the Serial Monitor.

Repeat these steps for each Arduino you want to use (e.g., one for "Hype" and one for "Blunder").

### Step 4: Set Up the PowerShell Script on Your Computer
The PowerShell script (`clipmanager.ps1`) listens for button presses and sends requests to the ClipManager server. We’ll configure it to start automatically when your computer boots.

#### 4.1 Copy the Shortcut to the Startup Folder
1. Open the Startup folder:
   - Press `Win + R` to open the "Run" dialog.
   - Type `shell:startup` and press Enter.
2. Copy the file `arduino_button/shortcut_clipmanager_run.bat` to the Startup folder.
   - This ensures the script starts automatically when your computer boots.

#### 4.2 Start the Script for the First Time
- Double-click `arduino_button/shortcut_clipmanager_run.bat` to start the script.
- Alternatively, restart your computer, and the script will start automatically.

### Step 5: Use the Button Integration
1. Ensure your Arduino(s) are connected via USB.
2. Press the button on an Arduino:
   - For example, if an Arduino is set with `IDENTIFIER = "CLIPMANAGER_HYPE"`, a clip with the category "Hype" will be sent to your configured chat platform.
   - If an Arduino is set with `IDENTIFIER = "CLIPMANAGER_BLUNDER"`, a clip with the category "Blunder" will be sent.
3. Check your chat platform (e.g., Mattermost, Telegram, or Discord) to see your clip in the configured channel. The clip message will include the team names and additional text if specified in `config.json`.

---

## How It Works
- The Arduino sends a signal ("BUTTON_PRESSED") to your computer when the button is pressed.
- The PowerShell script (`clipmanager.ps1`) detects this signal and sends a request to the ClipManager server to create a clip.
- The clip is sent to the chat platform specified in `config.json` with the category set in the Arduino code (e.g., "Hype" or "Blunder"). If provided, the team names (`Team1`, `Team2`) and additional text (`AdditionalText`) are included in the clip message.

---

## Troubleshooting
- **The script doesn’t start or shows an error**:
  - Check the error message in the PowerShell window. It will guide you on how to fix issues with `config.json` (e.g., missing `ServerUrl`, `ChatApp`, or chat platform credentials).
  - Ensure `shortcut_clipmanager_run.bat` is in the Startup folder (`shell:startup`).
  - Verify that your Arduino is connected before your computer boots.
- **The button doesn’t work**:
  - Check if the Arduino is programmed correctly (see Step 3).
  - Open Device Manager (`Win + X > Device Manager`) and confirm your Arduino is recognized (e.g., COM3 or COM4).
  - Test the button in the Arduino IDE Serial Monitor (see Step 3.4).
- **No clips in the chat platform**:
  - Ensure your computer has an internet connection.
  - Verify the `ServerUrl` in `config.json` (see Step 2).
  - Check if the chat platform credentials in `config.json` are correct (e.g., Mattermost token, Telegram bot token, or Discord webhook URL).
  - Ensure the ClipManager server is running and accessible.
- **Team names or additional text not appearing**:
  - Verify that `Team1`, `Team2`, and `AdditionalText` are correctly set in `config.json` (see Step 2).
  - Ensure the values are not empty if you want them to appear in the clip message.
- **I want to add a new category**:
  - Set a new identifier on your Arduino, e.g., `IDENTIFIER = "CLIPMANAGER_TEST"` for the category "Test". No changes to the script or `config.json` are needed; the category is automatically determined.
- **I want to use a different chat platform**:
  - Update the `ChatApp` and `ChatAppConfig` fields in `config.json` to match your desired platform (see Step 2 for examples).

---

## Technical Details for Developers
- **PowerShell Script (`clipmanager.ps1`)**:
  - The script listens for Arduino signals on serial ports and sends HTTP requests to the ClipManager server.
  - Configuration is read from `config.json`:
    - `ServerUrl`: The ClipManager server URL.
    - `ChatApp`: The chat platform to send clips to (`mattermost`, `telegram`, or `discord`).
    - `ChatAppConfig`: Platform-specific credentials.
    - `BacktrackSeconds` and `DurationSeconds`: Clip recording parameters.
    - `Team1` and `Team2`: Optional team names for sports clips.
    - `AdditionalText`: Optional additional description text for the clip message.
  - The script validates all required fields and provides clear error messages if something is missing.
  - If the specified `ServerUrl` is not reachable, it attempts to use `http://localhost:5001` as a fallback.
- **Arduino Code (`ClipManagerButton.ino`)**:
  - Uses a button on pin 12 with `INPUT_PULLUP` to detect presses.
  - Sends "BUTTON_PRESSED" over the serial connection when the button is pressed.
  - Responds to "IDENTIFY" requests with the configured identifier (e.g., `CLIPMANAGER_HYPE`).

For more details on the ClipManager server, see `DEVELOPER.md`.