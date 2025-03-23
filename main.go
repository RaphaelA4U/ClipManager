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
	"strconv"
	"strings"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
	"golang.org/x/time/rate"
)

type ClipRequest struct {
	// Common parameters (ordered logically)
	CameraIP         string `json:"camera_ip"`
	BacktrackSeconds int    `json:"backtrack_seconds"`
	DurationSeconds  int    `json:"duration_seconds"`
	ChatApp          string `json:"chat_app"`
	Category         string `json:"category"` // New optional category parameter
	
	// Chat app specific parameters
	// Telegram parameters
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`
	
	// Mattermost parameters
	MattermostURL     string `json:"mattermost_url"`     // e.g. https://mattermost.example.com
	MattermostToken   string `json:"mattermost_token"`   
	MattermostChannel string `json:"mattermost_channel"` 
	
	// Discord parameters
	DiscordWebhookURL string `json:"discord_webhook_url"`

	// PoolManager integration
	PoolManagerConnection bool `json:"poolmanager_connection"`
}

type ClipResponse struct {
	Message string `json:"message"`
}

// PoolManagerData represents data retrieved from the PoolManager API
type PoolManagerData struct {
	Players     []string `json:"players"`
	MatchNumber int      `json:"match_number"`
}

// Global rate limiter - 1 request per second with a burst of 1
var limiter = rate.NewLimiter(rate.Limit(1), 1)

// rateLimit is a middleware that limits requests to 1 per second
func rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			// Rate limit exceeded
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			log.Printf("Rate limit exceeded for IP: %s", r.RemoteAddr)
			return
		}
		// If rate limit not exceeded, proceed to the handler
		next(w, r)
	}
}

func main() {
	// Simple starting message
	log.Println("Starting ClipManager...")
	
	// Get internal port (what the app listens on)
	containerPort := getPort()
	
	// Get external port (what users connect to)
	hostPort := getHostPort(containerPort)
	
	// Use the host port for all user-facing URLs
	accessPort := hostPort
	
	// Set up HTTP server with rate limiting middleware
	http.HandleFunc("/api/clip", rateLimit(handleClipRequest))

	// Simple startup success message
	log.Println("ClipManager is running!")
	
	// Clear access information with example
	log.Printf("Access the application at: http://localhost:%s/api/clip", accessPort)
	log.Printf("Example request: http://localhost:%s/api/clip?camera_ip=rtsp://username:password@camera-ip:port/path&backtrack_seconds=10&duration_seconds=10&chat_app=telegram&telegram_bot_token=YOUR_BOT_TOKEN&telegram_chat_id=YOUR_CHAT_ID", accessPort)
	
	// Start the server (no additional messaging needed here)
	log.Fatal(http.ListenAndServe(":"+containerPort, nil))
}

// getPort gets the PORT value from environment variable or returns the default
// Simplified to reduce unnecessary logging
func getPort() string {
	envPort := os.Getenv("PORT")
	if envPort != "" {
		return envPort
	}
	return "5000"
}

// getHostPort determines the external port that users should connect to
// Simplified to reduce unnecessary logging
func getHostPort(defaultPort string) string {
	hostPort := os.Getenv("HOST_PORT")
	if hostPort != "" {
		return hostPort
	}
	return "5001" // Changed from defaultPort to "5001"
}

// checkIfRunningInDocker checks if the application is running inside a Docker container
// Function kept for backend logic but we'll minimize logging of this information
func checkIfRunningInDocker() bool {
	// Method 1: Check for /.dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	
	// Method 2: Check for docker in cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		return bytes.Contains(data, []byte("docker"))
	}
	
	// Method 3: Check for PORT and HOST_PORT environment variables
	if os.Getenv("PORT") != "" && os.Getenv("HOST_PORT") != "" {
		return true
	}
	
	return false
}

func handleClipRequest(w http.ResponseWriter, r *http.Request) {
	// Accept both GET and POST
	var req ClipRequest

	if r.Method == http.MethodGet {
		// Parse query parameters for GET (in logical order)
		req.CameraIP = r.URL.Query().Get("camera_ip")
		backtrackSeconds := r.URL.Query().Get("backtrack_seconds")
		durationSeconds := r.URL.Query().Get("duration_seconds")
		req.ChatApp = strings.ToLower(r.URL.Query().Get("chat_app"))
		req.Category = r.URL.Query().Get("category") // Parse category parameter
		
		// Chat app specific parameters
		req.TelegramBotToken = r.URL.Query().Get("telegram_bot_token")
		req.TelegramChatID = r.URL.Query().Get("telegram_chat_id")
		req.MattermostURL = r.URL.Query().Get("mattermost_url")
		req.MattermostToken = r.URL.Query().Get("mattermost_token")
		req.MattermostChannel = r.URL.Query().Get("mattermost_channel")
		req.DiscordWebhookURL = r.URL.Query().Get("discord_webhook_url")

		// Parse PoolManager connection parameter
		poolManagerParam := r.URL.Query().Get("poolmanager_connection")
		if poolManagerParam != "" {
			var err error
			req.PoolManagerConnection, err = strconv.ParseBool(poolManagerParam)
			if err != nil {
				log.Printf("Invalid poolmanager_connection parameter: %v", err)
				req.PoolManagerConnection = false
			}
		}

		// Parse numeric parameters
		if backtrackSeconds != "" {
			fmt.Sscanf(backtrackSeconds, "%d", &req.BacktrackSeconds)
		}
		if durationSeconds != "" {
			fmt.Sscanf(durationSeconds, "%d", &req.DurationSeconds)
		}
	} else if r.Method == http.MethodPost {
		// Parse JSON body for POST
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		// Standardize chat app to lowercase
		req.ChatApp = strings.ToLower(req.ChatApp)
	} else {
		http.Error(w, "Method not allowed, use GET or POST", http.StatusMethodNotAllowed)
		return
	}

	// Validate common parameters
	if req.CameraIP == "" {
		http.Error(w, "Missing required parameter: camera_ip", http.StatusBadRequest)
		return
	}
	
	if req.ChatApp == "" {
		http.Error(w, "Missing required parameter: chat_app", http.StatusBadRequest)
		return
	}
	
	if req.BacktrackSeconds < 0 {
		http.Error(w, "Invalid or missing parameter: backtrack_seconds must be 0 or greater", http.StatusBadRequest)
		return
	}
	
	if req.DurationSeconds <= 0 {
		http.Error(w, "Invalid or missing parameter: duration_seconds must be greater than 0", http.StatusBadRequest)
		return
	}
	
	if req.BacktrackSeconds > 300 {
		http.Error(w, "Invalid parameter: backtrack_seconds must be between 0 and 300", http.StatusBadRequest)
		return
	}
	
	if req.DurationSeconds < 1 || req.DurationSeconds > 300 {
		http.Error(w, "Invalid parameter: duration_seconds must be between 1 and 300", http.StatusBadRequest)
		return
	}

	// Chat app-specific validation
	switch req.ChatApp {
	case "telegram":
		if req.TelegramBotToken == "" {
			http.Error(w, "Missing required parameter for Telegram: telegram_bot_token", http.StatusBadRequest)
			return
		}
		if req.TelegramChatID == "" {
			http.Error(w, "Missing required parameter for Telegram: telegram_chat_id", http.StatusBadRequest)
			return
		}
	case "mattermost":
		if req.MattermostURL == "" {
			http.Error(w, "Missing required parameter for Mattermost: mattermost_url", http.StatusBadRequest)
			return
		}
		if req.MattermostToken == "" {
			http.Error(w, "Missing required parameter for Mattermost: mattermost_token", http.StatusBadRequest)
			return
		}
		if req.MattermostChannel == "" {
			http.Error(w, "Missing required parameter for Mattermost: mattermost_channel", http.StatusBadRequest)
			return
		}
		// Make sure MattermostURL has no trailing slash
		req.MattermostURL = strings.TrimSuffix(req.MattermostURL, "/")
	case "discord":
		if req.DiscordWebhookURL == "" {
			http.Error(w, "Missing required parameter for Discord: discord_webhook_url", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "Invalid chat_app parameter. Supported values are: 'telegram', 'mattermost', or 'discord'", http.StatusBadRequest)
		return
	}

	// Get data from PoolManager if needed
	var poolManagerData *PoolManagerData
	if req.PoolManagerConnection {
		poolManagerData = getPoolManagerData()
		if poolManagerData != nil {
			log.Printf("Retrieved PoolManager data: players=%v, match number=%d", 
				poolManagerData.Players, poolManagerData.MatchNumber)
		}
	}

	// Create a temporary directory for the clip
	tempDir := "clips"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("Failed to create directory %s: %v", tempDir, err)
		http.Error(w, "Server error: could not create temporary directory", http.StatusInternalServerError)
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
    
    // Log the command
    log.Printf("FFmpeg command: %s", ffmpegCmd.String())
    
    err := ffmpegCmd.Run()
    if err != nil {
        log.Printf("FFmpeg error: %v", err)
        http.Error(w, "Could not record the clip: RTSP stream may be unavailable or invalid", http.StatusInternalServerError)
        return
    }

	// Check if the file exists and is not too small
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("File stat error: %v", err)
		http.Error(w, "Could not access the recorded clip file", http.StatusInternalServerError)
		return
	}
	
	if fileInfo.Size() < 1024 {
		os.Remove(filePath) // Remove the file in case of error
		http.Error(w, "Recorded clip is too small, possibly no valid data received from the camera", http.StatusInternalServerError)
		return
	}

	// Check file size and compress only if > 50MB
	finalFilePath := filePath
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
				
				// Use the compressed file and remove the original
				os.Remove(filePath)
				finalFilePath = compressedFilePath
			} else {
				log.Printf("Error checking compressed file: %v, falling back to original", err)
			}
		}
	}

	// Send the clip to the chosen chat app (asynchronously)
	go func() {
		defer os.Remove(finalFilePath) // Make sure the file is always removed

		switch req.ChatApp {
		case "telegram":
			sendToTelegram(finalFilePath, req.TelegramBotToken, req.TelegramChatID, req.Category)
		case "mattermost":
			sendToMattermost(finalFilePath, req.MattermostURL, req.MattermostToken, req.MattermostChannel, req.Category)
		case "discord":
			sendToDiscord(finalFilePath, req.DiscordWebhookURL, req.Category)
		}
	}()

	// Send success response immediately
	response := ClipResponse{Message: "Clip recorded and sending started"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Function to send to Telegram
func sendToTelegram(filePath, botToken, chatID string, category string) {
    file, err := os.Open(filePath)
    if err != nil {
        log.Printf("Could not open file for sending to Telegram: %v", err)
        return
    }
    defer file.Close()

    // Generate message with optional category
    var captionText string
    if category != "" {
        captionText = fmt.Sprintf("New %s Clip: %s", category, formatCurrentTime())
    } else {
        captionText = fmt.Sprintf("New Clip: %s", formatCurrentTime())
    }

    // Make sure the telegram_chat_id is properly formatted (remove any quotes)
    chatID = strings.Trim(chatID, `"'`)
    
    // Ensure telegram_chat_id is not empty
    if chatID == "" {
        log.Printf("Error: telegram_chat_id is empty, cannot send to Telegram")
        return
    }
    
    reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendVideo", botToken)
    client := &http.Client{Timeout: 60 * time.Second} // Increased timeout for large files
    
    log.Printf("Sending clip to Telegram. File: %s", filepath.Base(filePath))

    // Create the multipart form
    var requestBody bytes.Buffer
    writer := multipart.NewWriter(&requestBody)
    
    // Add the chat_id field
    if err := writer.WriteField("chat_id", chatID); err != nil {
        log.Printf("Error preparing Telegram request: %v", err)
        return
    }
    
    // Add the caption field
    if err := writer.WriteField("caption", captionText); err != nil {
        log.Printf("Error adding caption to Telegram request: %v", err)
        return
    }
    
    // Add the video file
    part, err := writer.CreateFormFile("video", filepath.Base(filePath))
    if err != nil {
        log.Printf("Error creating file field for Telegram: %v", err)
        return
    }
    
    if _, err := io.Copy(part, file); err != nil {
        log.Printf("Error copying file to Telegram request: %v", err)
        return
    }
    
    // Close the writer
    if err := writer.Close(); err != nil {
        log.Printf("Error finalizing Telegram request: %v", err)
        return
    }

    // Create an HTTP POST request
    req, err := http.NewRequest("POST", reqURL, &requestBody)
    if err != nil {
        log.Printf("Error creating Telegram request: %v", err)
        return
    }
    
    // Set the content type
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    // Execute the request
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Error sending clip to Telegram: %v", err)
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

    log.Printf("Clip successfully sent to Telegram")
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
func sendToMattermost(filePath, mattermostURL, token, channelID string, category string) {
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
        log.Printf("Error preparing Mattermost request: %v", err)
        return
    }
    
    // Add the file
    part, err := writer.CreateFormFile("files", filepath.Base(filePath))
    if err != nil {
        log.Printf("Error creating file field for Mattermost: %v", err)
        return
    }
    
    if _, err := io.Copy(part, file); err != nil {
        log.Printf("Error copying file to Mattermost request: %v", err)
        return
    }
    
    // Close the writer
    if err := writer.Close(); err != nil {
        log.Printf("Error finalizing Mattermost request: %v", err)
        return
    }

    // First, upload the file
    fileUploadURL := fmt.Sprintf("%s/api/v4/files", mattermostURL)
    log.Printf("Uploading file to Mattermost")
    
    req, err := http.NewRequest("POST", fileUploadURL, &requestBody)
    if err != nil {
        log.Printf("Error creating Mattermost upload request: %v", err)
        return
    }
    
    req.Header.Set("Content-Type", writer.FormDataContentType())
    req.Header.Set("Authorization", "Bearer "+token)
    
    client := &http.Client{Timeout: 60 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Error uploading to Mattermost: %v", err)
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
    
    // Generate message with optional category
    var messageText string
    if category != "" {
        messageText = fmt.Sprintf("New %s Clip: %s", category, formatCurrentTime())
    } else {
        messageText = fmt.Sprintf("New Clip: %s", formatCurrentTime())
    }
    
    // Now create a post with the uploaded file
    fileIDs := make([]string, len(fileResponse.FileInfos))
    for i, fileInfo := range fileResponse.FileInfos {
        fileIDs[i] = fileInfo.ID
    }
    
    postData := map[string]interface{}{
        "channel_id": channelID,
        "message":    messageText,
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
        log.Printf("Error creating post request: %v", err)
        return
    }
    
    postReq.Header.Set("Content-Type", "application/json")
    postReq.Header.Set("Authorization", "Bearer "+token)
    
    postResp, err := client.Do(postReq)
    if err != nil {
        log.Printf("Error creating Mattermost post: %v", err)
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
func sendToDiscord(filePath, webhookURL string, category string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Could not open file for sending to Discord: %v", err)
		return
	}
	defer file.Close()

	// Generate message with optional category
	var messageText string
	if category != "" {
		messageText = fmt.Sprintf("New %s Clip: %s", category, formatCurrentTime())
	} else {
		messageText = fmt.Sprintf("New Clip: %s", formatCurrentTime())
	}

	// Create a multipart form for the file
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	
	// Add message text with timestamp
	if err := writer.WriteField("content", messageText); err != nil {
		log.Printf("Error adding content to Discord request: %v", err)
		return
	}
	
	// Add the file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		log.Printf("Error creating file field for Discord: %v", err)
		return
	}
	
	if _, err := io.Copy(part, file); err != nil {
		log.Printf("Error copying file to Discord request: %v", err)
		return
	}
	
	// Close the writer
	if err := writer.Close(); err != nil {
		log.Printf("Error finalizing Discord request: %v", err)
		return
	}

	log.Printf("Sending clip to Discord. File: %s", filepath.Base(filePath))

	// Create an HTTP POST request
	req, err := http.NewRequest("POST", webhookURL, &requestBody)
	if err != nil {
		log.Printf("Error creating Discord request: %v", err)
		return
	}
	
	// Set the content type
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Execute the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending to Discord: %v", err)
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

// getPoolManagerData returns simulated data from PoolManager API
func getPoolManagerData() *PoolManagerData {
	// Return simulated test data
	return &PoolManagerData{
		Players:     []string{"Alice", "Bob"},
		MatchNumber: 3,
	}
}