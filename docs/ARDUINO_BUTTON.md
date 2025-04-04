# ClipManager Button Integration (Optional)

This document explains how to set up the optional button integration for ClipManager. This feature allows you to trigger clip recordings by pressing a physical button connected to an Arduino, which communicates with a Windows PC or tablet to send requests to the ClipManager server.

---

## Overview

The button integration uses an Arduino with a push button to trigger clip recordings. When the button is pressed, a script running on a Windows PC sends a request to the ClipManager server to record and send a clip to a chat platform (e.g., Mattermost, Telegram, or Discord). The category of the clip (e.g., "Hype" or "Blunder") is determined by the Arduino's identifier.

### What You Need
- A Windows PC or tablet (Windows 10 or higher).
- An Arduino (e.g., Arduino Uno) with a push button connected to pin 12, already programmed with the ClipManager button code.
- A USB cable to connect the Arduino to your computer.
- An internet connection (to communicate with the ClipManager server).
- The ClipManager server already running (see `README.md` for setup instructions).
- Credentials for your chosen chat platform (e.g., Mattermost token, Telegram bot token, or Discord webhook URL).

---

## Setup Instructions

### Step 1: Download and Run the ClipManager Script
1. Navigate to the ClipManager script in your browser:
   - [View ClipManager Script](https://github.com/RaphaelA4U/ClipManager/blob/main/arduino_button/clipmanager.ps1)
2. Click the **Download raw file** button on the top-right of the code view.
3. Save the file (`clipmanager.ps1`) to your Downloads folder.
5. Right-click the saved file and select **Run with PowerShell**.
   - **Security Warning**: You may see a Windows security warning (e.g., "Windows protected your PC" or a User Account Control prompt) because the script is downloaded from the internet.
   - **Approve the Warning**:
     - If you see a "Windows protected your PC" message (SmartScreen), click **More info** and then **Run anyway**.
     - If you see a User Account Control (UAC) prompt asking for permission, click **Yes** to allow the script to run.

### Step 2: Configure the ClipManager Settings
1. The script will create a folder at `C:\ClipManager` and place the necessary files there.
2. A window will open in File Explorer showing the `C:\ClipManager` folder.
3. Open the file `config.json` in a text editor (e.g., Notepad).
4. Update the following fields with your settings:
   - **ServerUrl**: The URL of your ClipManager server (e.g., `"http://192.168.1.100:5001"`).
   - **ChatApp**: The chat platform to send clips to (`"mattermost"`, `"telegram"`, or `"discord"`).
   - **ChatAppConfig**: The credentials for your chat platform:
     - For Mattermost: `"mattermost_url"`, `"mattermost_channel"`, `"mattermost_token"`.
     - For Telegram: `"telegram_bot_token"`, `"telegram_chat_id"`.
     - For Discord: `"discord_webhook_url"`.
     - Leave unused fields empty (`""`).
   - **BacktrackSeconds** and **DurationSeconds**: The clip timing settings (default is `60` seconds each).
   - **Team1**, **Team2**, **AdditionalText**: Optional fields for sports clips (can be left empty).
5. Save the file after making changes.
6. Press any key in the script window to continue.

### Step 3: Complete the Setup
1. The script will complete the setup automatically:
   - It will copy itself to `C:\ClipManager`.
   - It will create a shortcut to run in the background.
   - It will place the shortcut in the Startup folder so it runs automatically after a PC restart.
   - It will start running in the background immediately.
2. You won’t see any windows after the setup because the script runs silently.
3. The script is now set to start automatically after every PC restart.

### Step 4: Use the Button Integration
1. Ensure your Arduino(s) are connected via USB.
2. Press the button on an Arduino:
   - For example, if an Arduino is set with the category "Hype", a clip with that category will be sent to your configured chat platform.
   - If an Arduino is set with the category "Blunder", a clip with that category will be sent.
3. Check your chat platform (e.g., Mattermost, Telegram, or Discord) to see your clip in the configured channel. The clip message will include the team names and additional text if specified in `config.json`.

---

## Updating the Script
If a new version of the ClipManager script is available:
1. Download the updated `clipmanager.ps1` from the same link provided in Step 1.
2. Double-click the new `clipmanager.ps1` to run it.
3. The script will detect that setup was already completed and skip the setup process, running the updated version immediately.
4. Your existing `config.json` settings will be preserved.

---

## How It Works
- The Arduino sends a signal ("BUTTON_PRESSED") to your computer when the button is pressed.
- The script (`clipmanager.ps1`) detects this signal and sends a request to the ClipManager server to create a clip.
- The clip is sent to the chat platform specified in `config.json` with the category set in the Arduino code (e.g., "Hype" or "Blunder"). If provided, the team names (`Team1`, `Team2`) and additional text (`AdditionalText`) are included in the clip message.
- The script runs in the background, invisible to the user, and continues running until the computer is shut down or the process is manually terminated via Task Manager.

---

## Troubleshooting
- **I see a security warning when running the script**:
  - This is normal the first time you run the script. Follow the instructions in Step 1 to approve the warning (e.g., click "More info" and "Run anyway" for SmartScreen, or "Yes" for UAC).
  - After approving the warning, the setup should proceed without issues.
- **The script doesn’t start after a restart**:
  - Open Task Manager (`Ctrl + Shift + Esc`) and check if `powershell.exe` is running. If not, run `C:\ClipManager\clipmanager.ps1` again to ensure all security warnings were approved.
  - Verify that `shortcut_clipmanager_run.bat` is in the Startup folder:
    - Press `Win + R`, type `shell:startup`, and press Enter.
    - Check if `shortcut_clipmanager_run.bat` is present.
- **No clips in the chat platform**:
  - Ensure your computer has an internet connection.
  - Verify the `ServerUrl` in `C:\ClipManager\config.json` is correct.
  - Check if the chat platform credentials in `config.json` are correct (e.g., Mattermost token, Telegram bot token, or Discord webhook URL).
  - Ensure the ClipManager server is running and accessible.
  - Confirm the script is running in the background by checking for `powershell.exe` in Task Manager.
- **Team names or additional text not appearing**:
  - Verify that `Team1`, `Team2`, and `AdditionalText` are correctly set in `C:\ClipManager\config.json`.
  - Ensure the values are not empty if you want them to appear in the clip message.
- **I want to change the settings**:
  - Open `C:\ClipManager\config.json` in a text editor, make your changes, and save the file.
  - Restart the script by ending the `powershell.exe` process in Task Manager (`Ctrl + Shift + Esc`) and running `C:\ClipManager\shortcut_clipmanager_run.bat`.
- **I want to stop the script**:
  - The script runs in the background and is not visible to the user. To stop it, open Task Manager (`Ctrl + Shift + Esc`), go to the **Processes** tab, find `powershell.exe`, right-click it, and select **End task**. Note that this requires some technical knowledge.

---

## Technical Details for Developers
- **PowerShell Script (`clipmanager.ps1`)**:
  - On first run, the script performs setup: creates `config.json`, prompts the user to edit it, creates `shortcut_clipmanager_run.bat`, copies the shortcut to the Startup folder, and starts itself in the background.
  - On subsequent runs, it skips the setup and runs the normal logic: listens for Arduino signals on serial ports and sends HTTP requests to the ClipManager server.
  - Configuration is read from `config.json`:
    - `ServerUrl`: The ClipManager server URL.
    - `ChatApp`: The chat platform to send clips to (`mattermost`, `telegram`, or `discord`).
    - `ChatAppConfig`: Platform-specific credentials.
    - `BacktrackSeconds` and `DurationSeconds`: Clip recording parameters.
    - `Team1` and `Team2`: Optional team names for sports clips.
    - `AdditionalText`: Optional additional description text for the clip message.
  - The script validates all required fields and provides clear error messages if something is missing.
- **Startup Script (`shortcut_clipmanager_run.bat`)**:
  - Created during setup and placed in `C:\ClipManager` and the Startup folder.
  - Ensures the script runs in the correct directory and launches the PowerShell script in a hidden window, so it runs silently in the background.

For more details on the ClipManager server, see `DEVELOPER.md`.