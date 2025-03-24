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
	// "strconv"  // Commented out since it's currently not used (PoolManager)
	"strings"
	"sync"
	"syscall"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
	"golang.org/x/time/rate"
	"github.com/joho/godotenv"
)

type ClipRequest struct {
	// Common parameters (ordered logically)
	CameraIP         string `json:"camera_ip"`
	BacktrackSeconds int    `json:"backtrack_seconds"`
	DurationSeconds  int    `json:"duration_seconds"`
	ChatApp          string `json:"chat_app"`
	Category         string `json:"category"`
	
	// Chat app specific parameters
	// Telegram parameters
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`
	
	// Mattermost parameters
	MattermostURL     string `json:"mattermost_url"`
	MattermostToken   string `json:"mattermost_token"`   
	MattermostChannel string `json:"mattermost_channel"` 
	
	// Discord parameters
	DiscordWebhookURL string `json:"discord_webhook_url"`

	// PoolManager integration - commented out but preserved for future use
	PoolManagerConnection bool `json:"poolmanager_connection"`
}

type ClipResponse struct {
	Message string `json:"message"`
}

// PoolManagerData represents data retrieved from the PoolManager API
// Commented out but preserved for future use
type PoolManagerData struct {
	Players     []string `json:"players"`
	MatchNumber int      `json:"match_number"`
}

// ClipManager handles clip recording, processing, and dispatch to chat apps
type ClipManager struct {
	tempDir    string
	httpClient *http.Client
	limiter    *rate.Limiter
	hostPort   string
	maxRetries int
	retryDelay time.Duration
	cameraIP   string
	bufferFile string // Path to the continuous recording buffer file
	recording  bool   // Flag to indicate if background recording is active
}

// NewClipManager creates a new ClipManager instance
func NewClipManager(tempDir string, hostPort string, cameraIP string) (*ClipManager, error) {
	// Ensure the temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %v", tempDir, err)
	}

	bufferFile := filepath.Join(tempDir, "buffer.mp4")

	return &ClipManager{
		tempDir:    tempDir,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		limiter:    rate.NewLimiter(rate.Limit(1), 1),
		hostPort:   hostPort,
		maxRetries: 3,
		retryDelay: 5 * time.Second,
		cameraIP:   cameraIP,
		bufferFile: bufferFile,
		recording:  false,
	}, nil
}

// RateLimit is a middleware that limits requests based on the ClipManager's rate limiter
func (cm *ClipManager) RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if (!cm.limiter.Allow()) {
			// Rate limit exceeded
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			log.Printf("Rate limit exceeded for IP: %s", r.RemoteAddr)
			return
		}
		// If rate limit not exceeded, proceed to the handler
		next(w, r)
	}
}

// HandleClipRequest handles HTTP requests to create and send clips
func (cm *ClipManager) HandleClipRequest(w http.ResponseWriter, r *http.Request) {
	// Track the start time for this request
	startTime := time.Now()
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	
	// Accept both GET and POST
	var req ClipRequest

	if r.Method == http.MethodGet {
		// Parse query parameters for GET (in logical order)
		req.CameraIP = r.URL.Query().Get("camera_ip")
		backtrackSeconds := r.URL.Query().Get("backtrack_seconds")
		durationSeconds := r.URL.Query().Get("duration_seconds")
		req.ChatApp = strings.ToLower(r.URL.Query().Get("chat_app"))
		req.Category = r.URL.Query().Get("category")
		
		// Chat app specific parameters
		req.TelegramBotToken = r.URL.Query().Get("telegram_bot_token")
		req.TelegramChatID = r.URL.Query().Get("telegram_chat_id")
		req.MattermostURL = r.URL.Query().Get("mattermost_url")
		req.MattermostToken = r.URL.Query().Get("mattermost_token")
		req.MattermostChannel = r.URL.Query().Get("mattermost_channel")
		req.DiscordWebhookURL = r.URL.Query().Get("discord_webhook_url")

		// Parse PoolManager connection parameter - commented out but preserved for future use
		/*
		poolManagerParam := r.URL.Query().Get("poolmanager_connection")
		if poolManagerParam != "" {
			var err error
			req.PoolManagerConnection, err = strconv.ParseBool(poolManagerParam)
			if err != nil {
				log.Printf("[%s] Invalid poolmanager_connection parameter: %v", requestID, err)
				req.PoolManagerConnection = false
			}
		}
		*/

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
			// Keep the chat_app field in its original form to support comma-separated values
		}
	} else {
		http.Error(w, "Method not allowed, use GET or POST", http.StatusMethodNotAllowed)
		return
	}

	// Validate common parameters
	if err := cm.validateRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a unique filename
	fileName := fmt.Sprintf("clip_%d.mp4", time.Now().Unix())
	filePath := filepath.Join(cm.tempDir, fileName)

	// Return response immediately after validation
	response := ClipResponse{Message: "Clip recording and sending started"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	// Process everything else asynchronously
	go func() {
		defer func() {
			// Log the total processing time when all operations are complete
			processingTime := time.Since(startTime)
			log.Printf("[%s] Total processing time: %v", requestID, processingTime)
		}()

		// Get data from PoolManager if needed - commented out but preserved for future use
		/*
		var poolManagerData *PoolManagerData
		if req.PoolManagerConnection {
			poolManagerData = cm.getPoolManagerData()
			if poolManagerData != nil {
				log.Printf("[%s] Retrieved PoolManager data: players=%v, match number=%d", 
					requestID, poolManagerData.Players, poolManagerData.MatchNumber)
			}
		}
		*/
		
		// Record the clip from the buffer file instead of directly from the camera
		log.Printf("[%s] Extracting clip for backtrack: %d seconds, duration: %d seconds", 
			requestID, req.BacktrackSeconds, req.DurationSeconds)
		err := cm.RecordClip(req.BacktrackSeconds, req.DurationSeconds, filePath)
		if err != nil {
			log.Printf("[%s] Recording error: %v", requestID, err)
			return
		}
		log.Printf("[%s] Clip recording completed", requestID)

		// Check file size and compress if needed
		finalFilePath, err := cm.CompressClipIfNeeded(filePath)
		if err != nil {
			log.Printf("[%s] Compression error: %v", requestID, err)
			// Clean up the original file if compression failed
			os.Remove(filePath)
			return
		}

		// Send the clip to the chosen chat apps
		if err := cm.SendToChatApp(finalFilePath, req); err != nil {
			log.Printf("[%s] Error sending clip: %v", requestID, err)
		}
		
		// Clean up the file after sending
		os.Remove(finalFilePath)
	}()
}

// validateRequest validates the clip request parameters
func (cm *ClipManager) validateRequest(req *ClipRequest) error {
	// Always use the camera IP from environment config
	req.CameraIP = cm.cameraIP
	
	if req.ChatApp == "" {
		return fmt.Errorf("missing required parameter: chat_app")
	}
	
	if req.BacktrackSeconds < 0 {
		return fmt.Errorf("invalid or missing parameter: backtrack_seconds must be 0 or greater")
	}
	
	if req.DurationSeconds <= 0 {
		return fmt.Errorf("invalid or missing parameter: duration_seconds must be greater than 0")
	}
	
	if req.BacktrackSeconds > 300 {
		return fmt.Errorf("invalid parameter: backtrack_seconds must be between 0 and 300")
	}
	
	if req.DurationSeconds < 1 || req.DurationSeconds > 300 {
		return fmt.Errorf("invalid parameter: duration_seconds must be between 1 and 300")
	}

	// Split the chat_app string into a list of chat apps
	chatApps := strings.Split(strings.ToLower(req.ChatApp), ",")
	
	// Validate each chat app
	for _, app := range chatApps {
		app = strings.TrimSpace(app)
		
		switch app {
		case "telegram":
			if req.TelegramBotToken == "" {
				return fmt.Errorf("missing required parameter for Telegram: telegram_bot_token")
			}
			if req.TelegramChatID == "" {
				return fmt.Errorf("missing required parameter for Telegram: telegram_chat_id")
			}
		case "mattermost":
			if req.MattermostURL == "" {
				return fmt.Errorf("missing required parameter for Mattermost: mattermost_url")
			}
			if req.MattermostToken == "" {
				return fmt.Errorf("missing required parameter for Mattermost: mattermost_token")
			}
			if req.MattermostChannel == "" {
				return fmt.Errorf("missing required parameter for Mattermost: mattermost_channel")
			}
			// Make sure MattermostURL has no trailing slash
			req.MattermostURL = strings.TrimSuffix(req.MattermostURL, "/")
		case "discord":
			if req.DiscordWebhookURL == "" {
				return fmt.Errorf("missing required parameter for Discord: discord_webhook_url")
			}
		default:
			return fmt.Errorf("invalid chat_app parameter '%s'. Supported values are: 'telegram', 'mattermost', or 'discord'", app)
		}
	}
	
	return nil
}

// StartBackgroundRecording starts a continuous recording of the RTSP stream in the background
// This creates a sliding window of footage that can be used for backtracking
func (cm *ClipManager) StartBackgroundRecording() {
	if cm.recording {
		log.Println("Background recording is already running")
		return
	}

	cm.recording = true
	
	log.Println("Starting background recording for backtracking capability...")
	
	// Create a separate goroutine for continuous recording
	go func() {
		attempt := 1
		for {
			// Check available disk space before starting a new recording cycle
			availableSpace, err := cm.CheckDiskSpace()
			if err != nil {
				log.Printf("Error checking disk space: %v, continuing with recording", err)
			} else {
				availableSpaceMB := availableSpace / (1024 * 1024)
				log.Printf("Available disk space: %d MB", availableSpaceMB)
				
				// If disk space is less than 500MB, skip this recording cycle
				if availableSpaceMB < 500 {
					log.Printf("Low disk space (< 500MB), skipping recording cycle, retrying in 30 seconds...")
					time.Sleep(30 * time.Second)
					continue
				}
			}
			
			// Set up FFmpeg command for continuous recording with a fixed duration of 300 seconds
			// Use copy codecs to maintain the original resolution (typically 1920x1080)
			outputArgs := ffmpeg.KwArgs{
				"t":          300, // 300 seconds (5 minutes) maximum backtrack window
				"c:v":        "copy", // Copy video codec to maintain 1920x1080 resolution
				"c:a":        "copy",
				"movflags":   "+faststart",
			}

			// Record to the buffer file
			ffmpegCmd := ffmpeg.Input(cm.cameraIP, ffmpeg.KwArgs{"rtsp_transport": "tcp"}).
				Output(cm.bufferFile, outputArgs).
				OverWriteOutput()
			
			// Log the command
			log.Printf("Background recording FFmpeg command: %s", ffmpegCmd.String())
			
			// Execute the FFmpeg command
			err = ffmpegCmd.Run()
			
			// Check if there was an error
			if err != nil {
				// If it's a connection error, retry after a delay
				if isConnectionError(err.Error()) {
					log.Printf("Camera disconnected, retrying connection (attempt %d)...", attempt)
					attempt++
					time.Sleep(10 * time.Second) // Wait 10 seconds before retrying
					continue
				}
				
				// Otherwise, log the error and continue with a new recording
				log.Printf("Background recording error: %v", err)
				time.Sleep(5 * time.Second) // Brief delay to avoid rapid retry loops
				attempt++
				continue
			}
			
			// If recording completed successfully, reset the attempt counter and start a new recording
			log.Println("Background recording cycle completed, starting next cycle...")
			attempt = 1
		}
	}()
}

// CheckDiskSpace returns the available disk space in bytes for the clips directory
func (cm *ClipManager) CheckDiskSpace() (uint64, error) {
	var stat syscall.Statfs_t
	
	// Get filesystem stats for the clips directory
	err := syscall.Statfs(cm.tempDir, &stat)
	if err != nil {
		return 0, fmt.Errorf("failed to get filesystem stats: %v", err)
	}
	
	// Calculate available space in bytes
	// Available blocks * size of block
	availableSpace := stat.Bavail * uint64(stat.Bsize)
	
	return availableSpace, nil
}

// RecordClip extracts a clip from the buffer file using the specified backtrack and duration
func (cm *ClipManager) RecordClip(backtrackSeconds, durationSeconds int, outputPath string) error {
	// Check if the buffer file exists and is valid
	fileInfo, err := os.Stat(cm.bufferFile)
	if err != nil {
		return fmt.Errorf("buffer file not available, camera may be disconnected: %v", err)
	}
	
	if fileInfo.Size() < 1024 {
		return fmt.Errorf("buffer file is too small, camera may be disconnected")
	}
	
	// Calculate the start time for the clip
	// Start from the end of the buffer file, going back by backtrackSeconds
	
	outputArgs := ffmpeg.KwArgs{
		"ss":         backtrackSeconds,       // Start position from the beginning of the buffer
		"t":          durationSeconds,        // Duration of the clip
		"c:v":        "copy",                 // Copy video codec to maintain 1920x1080 resolution
		"c:a":        "copy",                 // Copy audio codec
		"movflags":   "+faststart",           // Enable fast start for web playback
	}

	// Extract the clip from the buffer file
	ffmpegCmd := ffmpeg.Input(cm.bufferFile).
		Output(outputPath, outputArgs).
		OverWriteOutput()
	
	// Log the command
	log.Printf("Clip extraction FFmpeg command: %s", ffmpegCmd.String())
	
	// Execute the command
	err = ffmpegCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to extract clip from buffer: %v", err)
	}

	// Check if the extracted clip exists and is not too small
	fileInfo, err = os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("could not access the extracted clip file: %v", err)
	}
	
	if fileInfo.Size() < 1024 {
		os.Remove(outputPath)
		return fmt.Errorf("extracted clip is too small, possibly no valid data in the buffer for the specified time range")
	}
	
	return nil
}

// isConnectionError checks if an error message indicates a connection issue
func isConnectionError(errMsg string) bool {
	connectionErrors := []string{
		"connection refused",
		"Connection refused",
		"no route to host",
		"No route to host",
		"network is unreachable",
		"Network is unreachable",
		"connection timed out",
		"Connection timed out",
		"failed to connect",
		"EOF",
		"timeout",
		"Timeout",
	}
	
	for _, connErr := range connectionErrors {
		if strings.Contains(errMsg, connErr) {
			return true
		}
	}
	
	return false
}

// CompressClipIfNeeded checks if the clip needs compression and compresses it if necessary
func (cm *ClipManager) CompressClipIfNeeded(filePath string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("could not access the clip file: %v", err)
	}
	
	// Check file size and compress only if > 50MB
	finalFilePath := filePath
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	log.Printf("Original file size: %.2f MB", fileSizeMB)
	
	if fileInfo.Size() > 50*1024*1024 { // 50MB in bytes
		log.Printf("File is larger than 50MB (%.2f MB), applying compression", fileSizeMB)
		log.Printf("Reducing resolution to 1280x720 for compression")
		
		// Create path for compressed file
		compressedFilePath := filepath.Join(filepath.Dir(filePath), "compressed_"+filepath.Base(filePath))
		
		// Compress to 1280x720 with ultrafast preset (optimized for speed)
		compressCmd := ffmpeg.Input(filePath).
			Output(compressedFilePath, ffmpeg.KwArgs{
				"vf":       "scale=1280:720",  // Scale to 720p resolution for faster encoding
				"c:v":      "libx264",
				"preset":   "ultrafast",  // Fastest encoding (lower quality but much faster)
				"crf":      "28",         // Lower quality (higher number = lower quality)
				"c:a":      "aac",
				"b:a":      "96k",        // Lower audio bitrate
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
	
	return finalFilePath, nil
}

// RetryOperation executes the given function and retries up to maxRetries times if it fails
func (cm *ClipManager) RetryOperation(operation func() error, serviceName string) error {
	var err error
	
	// Try the main attempt
	err = operation()
	if err == nil {
		// Success on first try
		return nil
	}
	
	// Main attempt failed, log and start retries
	log.Printf("Error sending clip to %s: %v", serviceName, err)
	
	// Retry logic
	for attempt := 1; attempt <= cm.maxRetries; attempt++ {
		log.Printf("Retry %d/%d for %s...", attempt, cm.maxRetries, serviceName)
		
		// Wait before retrying
		time.Sleep(cm.retryDelay)
		
		// Try again
		err = operation()
		if err == nil {
			log.Printf("Retry %d/%d for %s succeeded", attempt, cm.maxRetries, serviceName)
			return nil
		}
		
		log.Printf("Retry %d/%d for %s failed: %v", attempt, cm.maxRetries, serviceName, err)
	}
	
	// All retries failed
	log.Printf("All %d retries failed for %s", cm.maxRetries, serviceName)
	return fmt.Errorf("failed to send clip to %s after %d attempts: %v", serviceName, cm.maxRetries+1, err)
}

// sendToTelegram sends a clip to Telegram
func (cm *ClipManager) sendToTelegram(filePath, botToken, chatID string, category string, poolManagerData *PoolManagerData) error {
	// Define the operation to be retried
	operation := func() error {
		file, err := os.Open(filePath)
		if (err != nil) {
			return fmt.Errorf("could not open file for sending to Telegram: %v", err)
		}
		defer file.Close()

		// Generate message with optional category and pool manager data
		var captionText string
		if category != "" {
			captionText = fmt.Sprintf("New %s Clip: %s", category, cm.formatCurrentTime())
		} else {
			captionText = fmt.Sprintf("New Clip: %s", cm.formatCurrentTime())
		}

		// Add team and match information if available - commented out but preserved for future use
		/*
		if poolManagerData != nil && len(poolManagerData.Players) == 2 {
			captionText += fmt.Sprintf(" - Teams: %s vs %s - Match: %d", 
				poolManagerData.Players[0], poolManagerData.Players[1], poolManagerData.MatchNumber)
		}
		*/

		// Make sure the telegram_chat_id is properly formatted (remove any quotes)
		chatID = strings.Trim(chatID, `"'`)
		
		// Ensure telegram_chat_id is not empty
		if chatID == "" {
			return fmt.Errorf("error: telegram_chat_id is empty, cannot send to Telegram")
		}
		
		reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendVideo", botToken)
		
		log.Printf("Sending clip to Telegram. File: %s", filepath.Base(filePath))

		// Create the multipart form
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		
		// Add the chat_id field
		if err := writer.WriteField("chat_id", chatID); err != nil {
			return fmt.Errorf("error preparing Telegram request: %v", err)
		}
		
		// Add the caption field
		if err := writer.WriteField("caption", captionText); err != nil {
			return fmt.Errorf("error adding caption to Telegram request: %v", err)
		}
		
		// Add the video file
		part, err := writer.CreateFormFile("video", filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("error creating file field for Telegram: %v", err)
		}
		
		if _, err := io.Copy(part, file); err != nil {
			return fmt.Errorf("error copying file to Telegram request: %v", err)
		}
		
		// Close the writer
		if err := writer.Close(); err != nil {
			return fmt.Errorf("error finalizing Telegram request: %v", err)
		}

		// Create an HTTP POST request
		req, err := http.NewRequest("POST", reqURL, &requestBody)
		if err != nil {
			return fmt.Errorf("error creating Telegram request: %v", err)
		}
		
		// Set the content type
		req.Header.Set("Content-Type", writer.FormDataContentType())
		
		// Execute the request
		resp, err := cm.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("error sending clip to Telegram: %v", err)
		}
		defer resp.Body.Close()

		// Read and log the response
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("telegram API error: %s - %s", resp.Status, responseBody)
		}

		log.Printf("Clip successfully sent to Telegram")
		return nil
	}
	
	// Execute the operation with retries
	return cm.RetryOperation(operation, "Telegram")
}

// sendToMattermost sends a clip to Mattermost
func (cm *ClipManager) sendToMattermost(filePath, mattermostURL, token, channelID string, category string, poolManagerData *PoolManagerData) error {
	// Define the operation to be retried
	operation := func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("could not open file for sending to Mattermost: %v", err)
		}
		defer file.Close()

		// Create a multipart form for the file upload
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		
		// Add the channel ID
		if err := writer.WriteField("channel_id", channelID); err != nil {
			return fmt.Errorf("error preparing Mattermost request: %v", err)
		}
		
		// Add the file
		part, err := writer.CreateFormFile("files", filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("error creating file field for Mattermost: %v", err)
		}
		
		if _, err := io.Copy(part, file); err != nil {
			return fmt.Errorf("error copying file to Mattermost request: %v", err)
		}
		
		// Close the writer
		if err := writer.Close(); err != nil {
			return fmt.Errorf("error finalizing Mattermost request: %v", err)
		}

		// First, upload the file
		fileUploadURL := fmt.Sprintf("%s/api/v4/files", mattermostURL)
		log.Printf("Uploading file to Mattermost")
		
		req, err := http.NewRequest("POST", fileUploadURL, &requestBody)
		if err != nil {
			return fmt.Errorf("error creating Mattermost upload request: %v", err)
		}
		
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		
		resp, err := cm.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("error uploading to Mattermost: %v", err)
		}
		defer resp.Body.Close()
		
		// Handle file upload response
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("mattermost file upload error: %s - %s", resp.Status, string(bodyBytes))
		}
		
		// Parse the response to get file IDs
		var fileResponse struct {
			FileInfos []struct {
				ID string `json:"id"`
			} `json:"file_infos"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&fileResponse); err != nil {
			return fmt.Errorf("error parsing Mattermost response: %v", err)
		}
		
		if len(fileResponse.FileInfos) == 0 {
			return fmt.Errorf("no file IDs returned from Mattermost")
		}
		
		// Generate message with optional category and pool manager data
		var messageText string
		if category != "" {
			messageText = fmt.Sprintf("New %s Clip: %s", category, cm.formatCurrentTime())
		} else {
			messageText = fmt.Sprintf("New Clip: %s", cm.formatCurrentTime())
		}
		
		// Add team and match information if available - commented out but preserved for future use
		/*
		if poolManagerData != nil && len(poolManagerData.Players) == 2 {
			messageText += fmt.Sprintf(" - Teams: %s vs %s - Match: %d", 
				poolManagerData.Players[0], poolManagerData.Players[1], poolManagerData.MatchNumber)
		}
		*/
		
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
			return fmt.Errorf("error creating post JSON: %v", err)
		}
		
		// Create the post
		postURL := fmt.Sprintf("%s/api/v4/posts", mattermostURL)
		postReq, err := http.NewRequest("POST", postURL, bytes.NewBuffer(postJSON))
		if err != nil {
			return fmt.Errorf("error creating post request: %v", err)
		}
		
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+token)
		
		postResp, err := cm.httpClient.Do(postReq)
		if err != nil {
			return fmt.Errorf("error creating Mattermost post: %v", err)
		}
		defer postResp.Body.Close()
		
		if postResp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(postResp.Body)
			return fmt.Errorf("mattermost post creation error: %s - %s", postResp.Status, string(bodyBytes))
		}
		
		log.Printf("Clip successfully sent to Mattermost")
		return nil
	}
	
	// Execute the operation with retries
	return cm.RetryOperation(operation, "Mattermost")
}

// sendToDiscord sends a clip to Discord
func (cm *ClipManager) sendToDiscord(filePath, webhookURL string, category string, poolManagerData *PoolManagerData) error {
	// Define the operation to be retried
	operation := func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("could not open file for sending to Discord: %v", err)
		}
		defer file.Close()

		// Generate message with optional category and pool manager data
		var messageText string
		if category != "" {
			messageText = fmt.Sprintf("New %s Clip: %s", category, cm.formatCurrentTime())
		} else {
			messageText = fmt.Sprintf("New Clip: %s", cm.formatCurrentTime())
		}

		// Add team and match information if available - commented out but preserved for future use
		/*
		if poolManagerData != nil && len(poolManagerData.Players) == 2 {
			messageText += fmt.Sprintf(" - Teams: %s vs %s - Match: %d", 
				poolManagerData.Players[0], poolManagerData.Players[1], poolManagerData.MatchNumber)
		}
		*/

		// Create a multipart form for the file
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		
		// Add message text with timestamp
		if err := writer.WriteField("content", messageText); err != nil {
			return fmt.Errorf("error adding content to Discord request: %v", err)
		}
		
		// Add the file
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("error creating file field for Discord: %v", err)
		}
		
		if _, err := io.Copy(part, file); err != nil {
			return fmt.Errorf("error copying file to Discord request: %v", err)
		}
		
		// Close the writer
		if err := writer.Close(); err != nil {
			return fmt.Errorf("error finalizing Discord request: %v", err)
		}

		log.Printf("Sending clip to Discord. File: %s", filepath.Base(filePath))

		// Create an HTTP POST request
		req, err := http.NewRequest("POST", webhookURL, &requestBody)
		if err != nil {
			return fmt.Errorf("error creating Discord request: %v", err)
		}
		
		// Set the content type
		req.Header.Set("Content-Type", writer.FormDataContentType())
		
		// Execute the request
		resp, err := cm.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("error sending to Discord: %v", err)
		}
		defer resp.Body.Close()
		
		// Check the response
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("discord API error: %s - %s", resp.Status, string(bodyBytes))
		}
		
		log.Printf("Clip successfully sent to Discord")
		return nil
	}
	
	// Execute the operation with retries
	return cm.RetryOperation(operation, "Discord")
}

// SendToChatApp sends the clip to the appropriate chat apps
func (cm *ClipManager) SendToChatApp(filePath string, req ClipRequest) error {
	// Retrieve PoolManager data if the connection is enabled - commented out but preserved for future use
	var poolManagerData *PoolManagerData = nil
	/*
	if req.PoolManagerConnection {
		poolManagerData = cm.getPoolManagerData()
	}
	*/

	// Split the chat_app string into a list of chat apps
	chatApps := strings.Split(strings.ToLower(req.ChatApp), ",")
	
	var wg sync.WaitGroup
	errors := make(chan error, len(chatApps))
	
	for _, app := range chatApps {
		app = strings.TrimSpace(app)
		
		wg.Add(1)
		go func(app string) {
			defer wg.Done()
			
			var err error
			switch app {
			case "telegram":
				err = cm.sendToTelegram(filePath, req.TelegramBotToken, req.TelegramChatID, req.Category, poolManagerData)
			case "mattermost":
				err = cm.sendToMattermost(filePath, req.MattermostURL, req.MattermostToken, req.MattermostChannel, req.Category, poolManagerData)
			case "discord":
				err = cm.sendToDiscord(filePath, req.DiscordWebhookURL, req.Category, poolManagerData)
			default:
				// This shouldn't happen since we validate earlier, but just in case
				err = fmt.Errorf("unsupported chat app: %s", app)
			}
			
			if err != nil {
				log.Printf("Error sending clip to %s: %v", app, err)
				errors <- fmt.Errorf("error sending to %s: %v", app, err)
			} else {
				log.Printf("Successfully sent clip to %s", app)
			}
		}(app)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)
	
	// Check if we had any errors
	var errList []string
	for err := range errors {
		errList = append(errList, err.Error())
	}
	
	if len(errList) > 0 {
		return fmt.Errorf("errors sending clip: %s", strings.Join(errList, "; "))
	}
	
	return nil
}

// formatCurrentTime returns a formatted current time string
func (cm *ClipManager) formatCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04")
}

// getPoolManagerData returns simulated data from PoolManager API
// Commented out but preserved for future use
/*
func (cm *ClipManager) getPoolManagerData() *PoolManagerData {
	// Return simulated test data
	// This is currently test data - in real implementation would fetch from PoolManager API
	return &PoolManagerData{
		Players:     []string{"Kylito & Raphael", "M4tthyTheSniper & 8BallJip"},
		MatchNumber: 3,
	}
}
*/

// getFileSize returns the size of a file in bytes
func (cm *ClipManager) getFileSize(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// checkIfRunningInDocker checks if the application is running inside Docker
func (cm *ClipManager) checkIfRunningInDocker() bool {
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

// serveWebInterface serves the HTML form interface at the root endpoint
func (cm *ClipManager) serveWebInterface(w http.ResponseWriter, r *http.Request) {
	// Define the path to the template file
	templatePath := "templates/index.html"
	
	// Check if the file exists
	_, err := os.Stat(templatePath)
	if err != nil {
		// If file doesn't exist, try to find it relative to the executable
		execPath, err := os.Executable()
		if err == nil {
			execDir := filepath.Dir(execPath)
			templatePath = filepath.Join(execDir, "templates/index.html")
		}
	}
	
	// Try to read the HTML file
	htmlContent, err := os.ReadFile(templatePath)
	if err != nil {
		// If we still can't find the file, use embedded HTML
		log.Printf("Error reading template file: %v, using embedded HTML", err)
		htmlContent = []byte(getEmbeddedHTML())
	}
	
	w.Header().Set("Content-Type", "text/html")
	w.Write(htmlContent)
}

// getEmbeddedHTML returns the HTML content as a fallback if the file can't be loaded
func getEmbeddedHTML() string {
	return `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ClipManager</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        h1 {
            color: #2c3e50;
            text-align: center;
        }
    </style>
</head>
<body>
    <h1>ClipManager</h1>
    <p>The template file could not be loaded. Please make sure the templates directory exists.</p>
    <p>API endpoint is still available at: /api/clip</p>
</body>
</html>
`
}

func main() {
	// Simple starting message
	log.Println("Starting ClipManager...")

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}
	
	// Get camera IP (required)
	cameraIP := os.Getenv("CAMERA_IP")
	if cameraIP == "" {
		log.Fatal("CAMERA_IP environment variable must be set")
	}
	
	// Get internal port (what the app listens on)
	containerPort := getPort()
	
	// Get external port (what users connect to)
	hostPort := getHostPort()
	if hostPort == "" {
		log.Fatal("HOST_PORT environment variable must be set")
	}
	
	// Create a new ClipManager instance
	clipManager, err := NewClipManager("clips", hostPort, cameraIP)
	if err != nil {
		log.Fatalf("Failed to initialize ClipManager: %v", err)
	}
	
	// Start the background recording for backtracking capability
	clipManager.StartBackgroundRecording()
	
	// Create necessary directories if they don't exist
	os.MkdirAll("templates", 0755)
	os.MkdirAll("static/css", 0755)
	os.MkdirAll("static/img", 0755)
	
	// Serve static files from the static directory
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	
	// Set up HTTP routes
	http.HandleFunc("/api/clip", clipManager.RateLimit(clipManager.HandleClipRequest))
	http.HandleFunc("/", clipManager.serveWebInterface)

	// Log startup information
	log.Println("ClipManager is running!")
	log.Printf("Access the web interface at: http://localhost:%s/", hostPort)
	log.Printf("API endpoint available at: http://localhost:%s/api/clip", hostPort)
	
	// Start the HTTP server
	log.Fatal(http.ListenAndServe(":"+containerPort, nil))
}

// getPort gets the PORT value from environment variable or returns the default
func getPort() string {
	envPort := os.Getenv("PORT")
	if (envPort != "") {
		return envPort
	}
	return "5000"
}

// getHostPort determines the external port that users should connect to
func getHostPort() string {
    hostPort := os.Getenv("HOST_PORT")
    if hostPort == "" {
        return "5001" // Default to 5001 if not specified
    }
    return hostPort
}