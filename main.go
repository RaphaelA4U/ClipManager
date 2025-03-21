package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type ClipRequest struct {
	CameraIP         string `json:"camera_ip"`
	ChatApp          string `json:"chat_app"`
	BotToken         string `json:"telegram_bot_token"` // For Telegram
	ChatID           string `json:"telegram_chat_id"`   // For Telegram
	MattermostURL    string `json:"mattermost_url"`     // For Mattermost (e.g. https://mattermost.example.com)
	MattermostToken  string `json:"mattermost_token"`  	// For Mattermost API token
	MattermostChannel string `json:"mattermost_channel"`// For Mattermost channel ID
	DiscordWebhookURL string `json:"discord_webhook_url"` // For Discord
	BacktrackSeconds int    `json:"backtrack_seconds"`
	DurationSeconds  int    `json:"duration_seconds"`
}

type ClipResponse struct {
	Message string `json:"message"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using default values")
	}

	// Get port from .env, default is 5000
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	// Set up HTTP server
	http.HandleFunc("/api/clip", handleClipRequest)

	// Log startup message
	log.Printf("ClipManager started! Make a GET/POST request to localhost:%s/api/clip", port)

	// Start the server
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleClipRequest(w http.ResponseWriter, r *http.Request) {
	// Accept both GET and POST
	var req ClipRequest

	if r.Method == http.MethodGet {
		// Parse query parameters for GET
		req.CameraIP = r.URL.Query().Get("camera_ip")
		req.ChatApp = r.URL.Query().Get("chat_app")
		req.BotToken = r.URL.Query().Get("telegram_bot_token")
		req.ChatID = r.URL.Query().Get("telegram_chat_id")
		req.MattermostURL = r.URL.Query().Get("mattermost_url")
		req.MattermostToken = r.URL.Query().Get("mattermost_token")
		req.MattermostChannel = r.URL.Query().Get("mattermost_channel")
		backtrackSeconds := r.URL.Query().Get("backtrack_seconds")
		durationSeconds := r.URL.Query().Get("duration_seconds")

		if backtrackSeconds != "" {
			fmt.Sscanf(backtrackSeconds, "%d", &req.BacktrackSeconds)
		}
		if durationSeconds != "" {
			fmt.Sscanf(durationSeconds, "%d", &req.DurationSeconds)
		}
	} else if r.Method == http.MethodPost {
		// Parse JSON body for POST
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "Method not allowed, use GET or POST", http.StatusMethodNotAllowed)
		return
	}

	// Validate common parameters
	if req.CameraIP == "" || req.ChatApp == "" {
		http.Error(w, "Missing parameters: camera_ip and chat_app are required", http.StatusBadRequest)
		return
	}
	if req.BacktrackSeconds < 5 || req.BacktrackSeconds > 300 {
		http.Error(w, "backtrack_seconds must be between 5 and 300", http.StatusBadRequest)
		return
	}
	if req.DurationSeconds < 5 || req.DurationSeconds > 300 {
		http.Error(w, "duration_seconds must be between 5 and 300", http.StatusBadRequest)
		return
	}

	// Chat app-specific validation
	req.ChatApp = strings.ToLower(req.ChatApp)
	switch req.ChatApp {
	case "telegram":
		if req.BotToken == "" || req.ChatID == "" {
			http.Error(w, "For Telegram, telegram_bot_token and telegram_chat_id are required", http.StatusBadRequest)
			return
		}
	case "mattermost":
		if req.MattermostURL == "" || req.MattermostToken == "" || req.MattermostChannel == "" {
			http.Error(w, "For Mattermost, mattermost_url, mattermost_token and mattermost_channel are required", http.StatusBadRequest)
			return
		}
		// Make sure MattermostURL has no trailing slash
		req.MattermostURL = strings.TrimSuffix(req.MattermostURL, "/")
	case "discord":
		if req.DiscordWebhookURL == "" {
			http.Error(w, "For Discord, discord_webhook_url is required", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "Only 'telegram', 'mattermost', and 'discord' are supported as chat_app", http.StatusBadRequest)
		return
	}

	// Create a temporary directory for the clip
	tempDir := "clips"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		http.Error(w, "Could not create temporary directory", http.StatusInternalServerError)
		return
	}

	// Generate a unique filename
	fileName := fmt.Sprintf("clip_%d.mp4", time.Now().Unix())
	filePath := filepath.Join(tempDir, fileName)
	compressedFilePath := filepath.Join(tempDir, "compressed_"+fileName)

	// First record the clip without compression
	outputArgs := ffmpeg.KwArgs{
		"ss":         req.BacktrackSeconds,
		"t":          req.DurationSeconds,
		"c:v":        "copy",  // Copy video without re-encoding
		"c:a":        "copy",  // Copy audio without re-encoding
		"movflags":   "+faststart",
	}

	// Record the clip with FFmpeg
    ffmpegCmd := ffmpeg.Input(req.CameraIP, ffmpeg.KwArgs{"rtsp_transport": "tcp"}).
        Output(filePath, outputArgs).
        OverWriteOutput()
    
    // Log the command (sanitized version)
    logCmd := sanitizeLogMessage(ffmpegCmd.String())
    log.Printf("FFmpeg command: %s", logCmd)
    
    err := ffmpegCmd.Run()
    if err != nil {
        log.Printf("FFmpeg error: %v", err)
        http.Error(w, "Could not record the clip", http.StatusInternalServerError)
        return
    }

	// Check if the file exists and is not too small
	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() < 1024 {
		os.Remove(filePath) // Remove the file in case of error
		http.Error(w, "Could not record the clip, file too small", http.StatusInternalServerError)
		return
	}

	// Check file size and compress only if > 50MB
	finalFilePath := filePath
	fileInfo, err = os.Stat(filePath)
	if err != nil {
		log.Printf("Could not get file information: %v", err)
		http.Error(w, "Could not process the clip", http.StatusInternalServerError)
		return
	}
    
    fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
    log.Printf("Original file size: %.2f MB", fileSizeMB)
    
    if fileInfo.Size() > 50*1024*1024 { // 50MB in bytes
        log.Printf("File is larger than 50MB (%.2f MB), applying compression", fileSizeMB)
        
        // Compress to 1920x1080 while maintaining aspect ratio
        compressCmd := ffmpeg.Input(filePath).
            Output(compressedFilePath, ffmpeg.KwArgs{
                "vf":       "scale=1920:-2",  // Scale to 1920px width, auto height to preserve aspect ratio
                "c:v":      "libx264",
                "preset":   "medium",  // Better quality than "ultrafast"
                "crf":      "23",      // Good quality (lower = better quality)
                "c:a":      "aac",
                "b:a":      "128k",
                "movflags": "+faststart",
            }). 
            OverWriteOutput()
            
        log.Printf("Compression command: %s", compressCmd.String())
        
        err = compressCmd.Run()
        if err != nil {
            log.Printf("Compression error: %v, using original file", err)
        } else {
            // Check compressed file size
            compressedInfo, err := os.Stat(compressedFilePath)
            if err == nil {
                compressedSizeMB := float64(compressedInfo.Size()) / 1024 / 1024
                log.Printf("Compressed file size: %.2f MB (%.1f%% of original)", 
                    compressedSizeMB, (compressedSizeMB/fileSizeMB)*100)
            }
            
            // Use the compressed file and remove the original
            os.Remove(filePath)
            finalFilePath = compressedFilePath
        }
    }

	// Send the clip to the chosen chat app (asynchronously)
	go func() {
		defer os.Remove(finalFilePath) // Make sure the file is always removed

		switch req.ChatApp {
		case "telegram":
			sendToTelegram(finalFilePath, req.BotToken, req.ChatID)
		case "mattermost":
			sendToMattermost(finalFilePath, req.MattermostURL, req.MattermostToken, req.MattermostChannel)
		case "discord":
			sendToDiscord(finalFilePath, req.DiscordWebhookURL)
		}
	}()

	// Send success response immediately
	response := ClipResponse{Message: "Clip recorded and sending started"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Function to send to Telegram
func sendToTelegram(filePath, botToken, chatID string) {
    file, err := os.Open(filePath)
    if err != nil {
        log.Printf("Could not open file for sending to Telegram: %v", err)
        return
    }
    defer file.Close()

    // Generate timestamp message
    captionText := fmt.Sprintf("New Clip: %s", formatCurrentTime())

    // Make sure the telegram_chat_id is properly formatted (remove any quotes)
    chatID = strings.Trim(chatID, `"'`)
    
    // Log the chat ID for debugging (sanitized)
    log.Printf("Sending to Telegram with telegram_chat_id length: %d", len(chatID))
    
    // Ensure telegram_chat_id is not empty
    if chatID == "" {
        log.Printf("Error: telegram_chat_id is empty, cannot send to Telegram")
        return
    }
    
    reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendVideo", botToken)
    client := &http.Client{Timeout: 60 * time.Second} // Increased timeout for large files
    
    // Debug logging (without sensitive data)
    log.Printf("Sending to Telegram. File path: %s, File size: %d bytes", 
        filepath.Base(filePath), getFileSize(filePath))

    // Create the multipart form
    var requestBody bytes.Buffer
    writer := multipart.NewWriter(&requestBody)
    
    // Add the telegram_chat_id field
    if err := writer.WriteField("telegram_chat_id", chatID); err != nil {
        log.Printf("Could not add telegram_chat_id to request: %v", err)
        return
    }
    
    // Add the caption field
    if err := writer.WriteField("caption", captionText); err != nil {
        log.Printf("Could not add caption to request: %v", err)
        return
    }
    
    // Add the video file
    part, err := writer.CreateFormFile("video", filepath.Base(filePath))
    if err != nil {
        log.Printf("Could not create file field: %v", err)
        return
    }
    
    if _, err := io.Copy(part, file); err != nil {
        log.Printf("Could not copy file to request: %v", err)
        return
    }
    
    // Close the writer
    if err := writer.Close(); err != nil {
        log.Printf("Could not close multipart writer: %v", err)
        return
    }

    // Create an HTTP POST request
    req, err := http.NewRequest("POST", reqURL, &requestBody)
    if err != nil {
        log.Printf("Could not create Telegram request: %v", err)
        return
    }
    
    // Set the content type
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    // Execute the request
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Error when sending to Telegram: %v", err)
        return
    }
    defer resp.Body.Close()

    // Read and log the response
    bodyBytes, _ := io.ReadAll(resp.Body)
    responseBody := string(bodyBytes)
    
    if resp.StatusCode != http.StatusOK {
        log.Printf("Telegram API error: %s - %s", resp.Status, responseBody)
        return
    }

    log.Printf("Clip successfully sent to Telegram: %s", responseBody)
}

// Helper function to get file size
func getFileSize(filePath string) int64 {
    info, err := os.Stat(filePath)
    if (err != nil) {
        return 0
    }
    return info.Size()
}

// Function to send to Mattermost
func sendToMattermost(filePath, mattermostURL, token, channelID string) {
    file, err := os.Open(filePath)
    if err != nil {
        log.Printf("Could not open file for sending to Mattermost: %v", err)
        return
    }
    defer file.Close()

    // Create a multipart form for the file upload
    var requestBody bytes.Buffer
    writer := multipart.NewWriter(&requestBody)
    
    // Add the channel ID
    if err := writer.WriteField("channel_id", channelID); err != nil {
        log.Printf("Could not add channel_id to request: %v", err)
        return
    }
    
    // Add the file
    part, err := writer.CreateFormFile("files", filepath.Base(filePath))
    if err != nil {
        log.Printf("Could not create file field: %v", err)
        return
    }
    
    if _, err := io.Copy(part, file); err != nil {
        log.Printf("Could not copy file to request: %v", err)
        return
    }
    
    // Close the writer
    if err := writer.Close(); err != nil {
        log.Printf("Could not close multipart writer: %v", err)
        return
    }

    // First, upload the file
    fileUploadURL := fmt.Sprintf("%s/api/v4/files", mattermostURL)
    log.Printf("Uploading file to Mattermost: %s", fileUploadURL)
    
    req, err := http.NewRequest("POST", fileUploadURL, &requestBody)
    if err != nil {
        log.Printf("Could not create Mattermost file upload request: %v", err)
        return
    }
    
    req.Header.Set("Content-Type", writer.FormDataContentType())
    req.Header.Set("Authorization", "Bearer "+token)
    
    client := &http.Client{Timeout: 60 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Error when uploading to Mattermost: %v", err)
        return
    }
    defer resp.Body.Close()
    
    // Handle file upload response
    if resp.StatusCode >= 300 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        log.Printf("Mattermost file upload error: %s - %s", resp.Status, string(bodyBytes))
        return
    }
    
    // Parse the response to get file IDs
    var fileResponse struct {
        FileInfos []struct {
            ID string `json:"id"`
        } `json:"file_infos"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&fileResponse); err != nil {
        log.Printf("Error parsing Mattermost response: %v", err)
        return
    }
    
    if len(fileResponse.FileInfos) == 0 {
        log.Printf("No file IDs returned from Mattermost")
        return
    }
    
    // Now create a post with the uploaded file
    fileIDs := make([]string, len(fileResponse.FileInfos))
    for i, fileInfo := range fileResponse.FileInfos {
        fileIDs[i] = fileInfo.ID
    }
    
    postData := map[string]interface{}{
        "channel_id": channelID,
        "message":    fmt.Sprintf("New Clip: %s", formatCurrentTime()),
        "file_ids":   fileIDs,
    }
    
    postJSON, err := json.Marshal(postData)
    if err != nil {
        log.Printf("Error creating post JSON: %v", err)
        return
    }
    
    // Create the post
    postURL := fmt.Sprintf("%s/api/v4/posts", mattermostURL)
    postReq, err := http.NewRequest("POST", postURL, bytes.NewBuffer(postJSON))
    if err != nil {
        log.Printf("Could not create post request: %v", err)
        return
    }
    
    postReq.Header.Set("Content-Type", "application/json")
    postReq.Header.Set("Authorization", "Bearer "+token)
    
    postResp, err := client.Do(postReq)
    if err != nil {
        log.Printf("Error creating post: %v", err)
        return
    }
    defer postResp.Body.Close()
    
    if postResp.StatusCode >= 300 {
        bodyBytes, _ := io.ReadAll(postResp.Body)
        log.Printf("Mattermost post creation error: %s - %s", postResp.Status, string(bodyBytes))
        return
    }
    
    log.Printf("Clip successfully sent to Mattermost")
}

// Function to send to Discord
func sendToDiscord(filePath, webhookURL string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Could not open file for sending to Discord: %v", err)
		return
	}
	defer file.Close()

	// Generate timestamp message
	messageText := fmt.Sprintf("New Clip: %s", formatCurrentTime())

	// Create a multipart form for the file
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	
	// Add message text with timestamp
	if err := writer.WriteField("content", messageText); err != nil {
		log.Printf("Could not add content to request: %v", err)
		return
	}
	
	// Add the file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		log.Printf("Could not create file field: %v", err)
		return
	}
	
	if _, err := io.Copy(part, file); err != nil {
		log.Printf("Could not copy file to request: %v", err)
		return
	}
	
	// Close the writer
	if err := writer.Close(); err != nil {
		log.Printf("Could not close multipart writer: %v", err)
		return
	}

	// Debug logging (without sensitive data)
	log.Printf("Sending to Discord. File path: %s, File size: %d bytes", 
		filepath.Base(filePath), getFileSize(filePath))

	// Create an HTTP POST request
	req, err := http.NewRequest("POST", webhookURL, &requestBody)
	if err != nil {
		log.Printf("Could not create Discord request: %v", err)
		return
	}
	
	// Set the content type
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Execute the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error when sending to Discord: %v", err)
		return
	}
	defer resp.Body.Close()
	
	// Check the response
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Discord API error: %s - %s", resp.Status, string(bodyBytes))
		return
	}
	
	log.Printf("Clip successfully sent to Discord")
}

// Helper to create multipart form data
type multipartForm struct {
	fields map[string]string
	files  map[string]*os.File
}

func (mf *multipartForm) Build() (body *os.File, contentType string, err error) {
	bodyFile, err := os.CreateTemp("", "multipart-*.tmp")
	if err != nil {
		return nil, "", err
	}

	writer := multipart.NewWriter(bodyFile)
	defer writer.Close()

	// Add fields
	for key, value := range mf.fields {
		if err := writer.WriteField(key, value); err != nil {
			bodyFile.Close()
			os.Remove(bodyFile.Name())
			return nil, "", err
		}
	}

	// Add files
	for key, file := range mf.files {
		part, err := writer.CreateFormFile(key, filepath.Base(file.Name()))
		if err != nil {
			bodyFile.Close()
			os.Remove(bodyFile.Name())
			return nil, "", err
		}
		if _, err := io.Copy(part, file); err != nil {
			bodyFile.Close()
			os.Remove(bodyFile.Name())
			return nil, "", err
		}
	}

	// Close the writer to write the boundary
	writer.Close()

	// Set the file pointer back to the beginning
	if _, err := bodyFile.Seek(0, 0); err != nil {
		bodyFile.Close()
		os.Remove(bodyFile.Name())
		return nil, "", err
	}

	return bodyFile, writer.FormDataContentType(), nil
}

// Helper function to format current date-time
func formatCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04")
}

// SanitizeLogMessage removes sensitive information from log messages
func sanitizeLogMessage(message string) string {
    // Hide camera IP/credentials
    re := regexp.MustCompile(`rtsp://[^@]+@[^/\s]+`)
    message = re.ReplaceAllString(message, "rtsp://[REDACTED]@[REDACTED]")
    
    // Hide Telegram bot tokens (format: 123456789:ABCDEFGhijklmnopqrstuvwxyz...)
    re = regexp.MustCompile(`\b\d+:[\w-]{35,}\b`)
    message = re.ReplaceAllString(message, "[REDACTED-BOT-TOKEN]")
    
    // Hide Telegram chat IDs
    re = regexp.MustCompile(`telegram_chat_id=(-?\d+)`)
    message = re.ReplaceAllString(message, "telegram_chat_id=[REDACTED-CHAT-ID]")
    
    // Hide Mattermost tokens
    re = regexp.MustCompile(`Bearer\s+[a-zA-Z0-9]+`)
    message = re.ReplaceAllString(message, "Bearer [REDACTED-TOKEN]")
    
    // Hide Discord webhook URLs
    re = regexp.MustCompile(`https://discord\.com/api/webhooks/[^/\s]+/[^/\s]+`)
    message = re.ReplaceAllString(message, "https://discord.com/api/webhooks/[REDACTED]/[REDACTED]")
    
    return message
}

type logSanitizer struct {
    out io.Writer
}

func (l *logSanitizer) Write(p []byte) (n int, err error) {
    sanitized := sanitizeLogMessage(string(p))
    return l.out.Write([]byte(sanitized))
}

func init() {
    // Use os.Stdout directly instead of logger.Writer()
    log.SetOutput(&logSanitizer{os.Stdout})
}