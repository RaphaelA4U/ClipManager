package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"
	"github.com/joho/godotenv"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
)

// Logger struct to handle custom logging
type Logger struct {
	logger *log.Logger
}

// NewLogger creates a new custom logger
func NewLogger() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Info logs an informational message (blue with ℹ️ emoji)
func (l *Logger) Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("%sℹ️  %s%s%s", Blue, Cyan, msg, Reset)
}

// Success logs a success message (green with ✅ emoji)
func (l *Logger) Success(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("%s✅ %s%s%s", Green, Green, msg, Reset)
}

// Warning logs a warning message (yellow with ⚠️ emoji)
func (l *Logger) Warning(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("%s⚠️  %s%s%s", Yellow, Yellow, msg, Reset)
}

// Error logs an error message (red with ❌ emoji)
func (l *Logger) Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("%s❌ %s%s%s", Red, Red, msg, Reset)
}

// Debug logs a debug message (cyan with 🔧 emoji)
func (l *Logger) Debug(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("%s🔧 %s%s%s", Cyan, Cyan, msg, Reset)
}

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

// SegmentInfo holds information about a recorded segment
type SegmentInfo struct {
	Path      string
	Timestamp time.Time
}

type ClipManager struct {
	tempDir         string
	httpClient      *http.Client
	limiter         *rate.Limiter
	hostPort        string
	maxRetries      int
	retryDelay      time.Duration
	cameraIP        string
	segmentPattern  string // Pattern for segment files
	recording       bool   // Flag to indicate if background recording is active
	segments        []SegmentInfo // List of available segments with timestamps
	segmentsMutex   sync.RWMutex  // Mutex for thread-safe segments list access
	segmentChan     chan SegmentInfo // Channel to receive new segments
	segmentDuration int // Duration of each segment in seconds
	logger          *Logger // Custom logger for colored and emoji logs
}

// NewClipManager creates a new ClipManager instance
func NewClipManager(tempDir string, hostPort string, cameraIP string) (*ClipManager, error) {
	// Ensure the temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %v", tempDir, err)
	}

	// Convert tempDir to absolute path to avoid path issues
	absTemp, err := filepath.Abs(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for tempDir: %v", err)
	}

	// Base segment pattern without the cycle suffix (will be added dynamically)
	segmentPattern := filepath.Join(absTemp, "segment_%03d.ts")

	return &ClipManager{
		tempDir:         absTemp,
		httpClient:      &http.Client{Timeout: 60 * time.Second},
		limiter:         rate.NewLimiter(rate.Limit(1), 1),
		hostPort:        hostPort,
		maxRetries:      3,
		retryDelay:      5 * time.Second,
		cameraIP:        cameraIP,
		segmentPattern:  segmentPattern,
		recording:       false,
		segments:        []SegmentInfo{},
		segmentsMutex:   sync.RWMutex{},
		segmentChan:     make(chan SegmentInfo, 100), // Buffered channel for new segments
		segmentDuration: 5, // Segment duration in seconds
		logger:          NewLogger(), // Initialize the custom logger
	}, nil
}

// RateLimit is a middleware that limits requests based on the ClipManager's rate limiter
func (cm *ClipManager) RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cm.limiter.Allow() {
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
	} else {
		http.Error(w, "Method not allowed, use GET or POST", http.StatusMethodNotAllowed)
		return
	}

	// Validate common parameters
	if err := cm.validateRequest(&req); err != nil {
		cm.logger.Error("Validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a unique filename with .mp4 extension
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
			cm.logger.Info("[%s] Total processing time: %v", requestID, processingTime)
		}()

		// Record the clip from the buffer file instead of directly from the camera
		cm.logger.Info("[%s] Extracting clip for backtrack: %d seconds, duration: %d seconds",
			requestID, req.BacktrackSeconds, req.DurationSeconds)
		err := cm.RecordClip(req.BacktrackSeconds, req.DurationSeconds, filePath, startTime)
		if err != nil {
			cm.logger.Error("[%s] Recording error: %v", requestID, err)
			return
		}
		cm.logger.Success("[%s] Clip recording completed", requestID)

		// Send the clip to the chosen chat apps
		if err := cm.SendToChatApp(filePath, req); err != nil {
			cm.logger.Error("[%s] Error sending clip: %v", requestID, err)
		}

		// Clean up the original file after sending
		os.Remove(filePath)
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

	if req.DurationSeconds > 300 {
		return fmt.Errorf("invalid parameter: duration_seconds must be less than 300")
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

// StartBackgroundRecording starts a continuous recording of the RTSP stream in segments
func (cm *ClipManager) StartBackgroundRecording() {
	if cm.recording {
		cm.logger.Warning("Background recording is already running")
		return
	}

	cm.recording = true

	cm.logger.Info("Starting background recording with segments for backtracking capability...")

	// Create a separate goroutine for continuous recording
	go func() {
		attempt := 1
		cycle := 0 // Counter for recording cycles to ensure unique segment names

		for {
			// Check available disk space before starting a new recording cycle
			availableSpace, err := cm.CheckDiskSpace()
			if err != nil {
				cm.logger.Error("Error checking disk space: %v, continuing with recording", err)
			} else {
				availableSpaceMB := availableSpace / (1024 * 1024)
				cm.logger.Info("Available disk space: %d MB", availableSpaceMB)

				// If disk space is less than 500MB, skip this recording cycle
				if availableSpaceMB < 500 {
					cm.logger.Warning("Low disk space (< 500MB), skipping recording cycle, retrying in 30 seconds...")
					time.Sleep(30 * time.Second)
					continue
				}
			}

			// Generate a unique segment pattern for this cycle
			segmentPattern := fmt.Sprintf("%s_cycle%d_%%03d.ts", strings.TrimSuffix(cm.segmentPattern, "_%03d.ts"), cycle)
			segmentList := filepath.Join(cm.tempDir, fmt.Sprintf("segments_cycle%d.m3u8", cycle))

			// Create FFmpeg command line
			args := []string{
				"-rtsp_transport", "tcp",
				"-i", cm.cameraIP,
				"-f", "segment",
				"-segment_time", "5", // 5-second segments
				"-segment_format", "mpegts",
				"-reset_timestamps", "1",
				"-segment_list", segmentList,
				"-segment_list_type", "m3u8",
				"-c:v", "copy", // Copy video codec to maintain original resolution
				"-c:a", "copy", // Copy audio codec
				"-y",           // Overwrite output files without asking
				segmentPattern,
			}

			// Log the command
			logCmd := fmt.Sprintf("ffmpeg %s", strings.Join(args, " "))
			cm.logger.Debug("Segment recording FFmpeg command: %s", logCmd)

			// Create the command
			cmd := exec.Command("ffmpeg", args...)

			// Get stderr pipe to monitor segment creation
			stderr, err := cmd.StderrPipe()
			if err != nil {
				cm.logger.Error("Error getting stderr pipe: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Start the command
			if err := cmd.Start(); err != nil {
				cm.logger.Error("Error starting FFmpeg: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Start a goroutine to scan stderr for segment creation messages
			go func(cycle int) {
				scanner := bufio.NewScanner(stderr)
				// Update regex to capture the filename with cycle suffix
				segmentRegex := regexp.MustCompile(fmt.Sprintf(`Opening '.*/(segment_cycle%d_\d+\.ts)' for writing`, cycle))

				for scanner.Scan() {
					line := scanner.Text()
					// Look for segment creation messages
					matches := segmentRegex.FindStringSubmatch(line)
					if len(matches) > 1 {
						segmentFile := matches[1]
						cm.logger.Success("New segment created: %s", segmentFile)

						// Add to segments list for backtracking
						cm.addSegment(segmentFile)
					}
				}

				if err := scanner.Err(); err != nil {
					cm.logger.Error("Error reading FFmpeg stderr: %v", err)
				}
			}(cycle)

			// Wait for the command to complete
			err = cmd.Wait()

			// Check if there was an error
			if err != nil {
				// Capture stderr output for better debugging
				stderrBytes, _ := io.ReadAll(stderr)
				errMsg := string(stderrBytes)
				cm.logger.Error("FFmpeg error: %v\nFFmpeg output: %s", err, errMsg)
				if isConnectionError(errMsg) {
					cm.logger.Warning("Camera disconnected, retrying connection (attempt %d)...", attempt)
					attempt++
					time.Sleep(10 * time.Second) // Wait 10 seconds before retrying
					continue
				}

				// Otherwise, log the error and continue with a new recording
				cm.logger.Error("Background recording error: %v", err)
				time.Sleep(5 * time.Second) // Brief delay to avoid rapid retry loops
				attempt++
				continue
			}

			// If recording completed successfully, reset the attempt counter and start a new cycle
			cm.logger.Info("Background recording cycle completed, starting next cycle...")
			attempt = 1
			cycle++ // Increment cycle counter for unique segment names
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

// addSegment adds a segment to the list of available segments
func (cm *ClipManager) addSegment(segmentPath string) {
	cm.segmentsMutex.Lock()
	defer cm.segmentsMutex.Unlock()

	// Construct the absolute path properly by joining tempDir and segment filename
	absolutePath := filepath.Join(cm.tempDir, segmentPath)

	// Create SegmentInfo with the current timestamp
	segmentInfo := SegmentInfo{
		Path:      absolutePath,
		Timestamp: time.Now(),
	}

	// Add the segment to the list
	cm.segments = append(cm.segments, segmentInfo)

	// Sort segments by timestamp to ensure chronological order
	sort.Slice(cm.segments, func(i, j int) bool {
		return cm.segments[i].Timestamp.Before(cm.segments[j].Timestamp)
	})

	// Keep only the last 62 segments (62 * 5 seconds = 310 seconds maximum backtrack)
	maxSegments := 62
	if len(cm.segments) > maxSegments {
		// Get the oldest segments to remove
		segmentsToRemove := cm.segments[:len(cm.segments)-maxSegments]

		// Update the segments list
		cm.segments = cm.segments[len(cm.segments)-maxSegments:]

		// Delete the old segment files
		for _, oldSegment := range segmentsToRemove {
			if _, err := os.Stat(oldSegment.Path); err == nil {
				if err := os.Remove(oldSegment.Path); err != nil {
					log.Printf("❌ Error removing old segment %s: %v", oldSegment.Path, err)
				} else {
					log.Printf("🗑️ Removed old segment: %s", oldSegment.Path)
				}
			}
		}
	}

	// Send the new segment to the channel for waiting routines
	cm.segmentChan <- segmentInfo

	log.Printf("📼 Current segments: %d (up to %d seconds of backtracking available)",
		len(cm.segments), len(cm.segments)*cm.segmentDuration)
}

// getVideoAspectRatio retrieves the aspect ratio of a video file using ffprobe
func (cm *ClipManager) getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "json",
		filePath)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe failed to get video dimensions: %v", err)
	}

	var result struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return "", fmt.Errorf("failed to parse ffprobe output: %v", err)
	}

	if len(result.Streams) == 0 {
		return "", fmt.Errorf("no video stream found in file")
	}

	width := result.Streams[0].Width
	height := result.Streams[0].Height

	if width == 0 || height == 0 {
		return "", fmt.Errorf("invalid video dimensions: width=%d, height=%d", width, height)
	}

	// Calculate the aspect ratio as a string (e.g., "16:9")
	gcd := func(a, b int) int {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}
	divisor := gcd(width, height)
	aspectRatio := fmt.Sprintf("%d:%d", width/divisor, height/divisor)

	return aspectRatio, nil
}

// RecordClip extracts a clip from the segments based on backtrack and duration
func (cm *ClipManager) RecordClip(backtrackSeconds, durationSeconds int, outputPath string, requestTime time.Time) error {
	// Calculate the desired start and end times
	startTime := requestTime.Add(-time.Duration(backtrackSeconds) * time.Second)
	endTime := startTime.Add(time.Duration(durationSeconds) * time.Second)

	log.Printf("📹 Requested clip from %s to %s", startTime.Format("15:04:05"), endTime.Format("15:04:05"))

	// Collect the required segments
	var neededSegments []SegmentInfo

	// Wait for segments to cover the entire time range
	for {
		// Get a copy of current segments
		cm.segmentsMutex.RLock()
		segments := make([]SegmentInfo, len(cm.segments))
		copy(segments, cm.segments)
		cm.segmentsMutex.RUnlock()

		if len(segments) == 0 {
			log.Println("⚠️ No segments available, waiting for first segment...")
			select {
			case newSegment := <-cm.segmentChan:
				log.Printf("📼 Received first segment: %s", newSegment.Path)
				continue
			case <-time.After(30 * time.Second):
				return fmt.Errorf("timeout waiting for first segment")
			}
		}

		// Find the segments that cover the requested time range
		neededSegments = []SegmentInfo{}
		earliestTime := segments[0].Timestamp
		latestTime := segments[len(segments)-1].Timestamp

		// Check if we have segments early enough for the start time
		if startTime.Before(earliestTime) {
			log.Printf("⚠️ Requested start time %s is before earliest segment at %s", startTime.Format("15:04:05"), earliestTime.Format("15:04:05"))
			// Adjust start time to the earliest available segment
			startTime = earliestTime
			endTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
			log.Printf("🔄 Adjusted clip time to %s to %s", startTime.Format("15:04:05"), endTime.Format("15:04:05"))
		}

		// Check if we need to wait for future segments
		if endTime.After(latestTime) {
			log.Printf("⏳ End time %s is after latest segment at %s, waiting for more segments...", endTime.Format("15:04:05"), latestTime.Format("15:04:05"))
			timeout := time.After(2 * time.Duration(durationSeconds) * time.Second) // Timeout after twice the duration
			select {
			case newSegment := <-cm.segmentChan:
				log.Printf("📼 Received new segment: %s at %s", newSegment.Path, newSegment.Timestamp.Format("15:04:05"))
				continue
			case <-timeout:
				return fmt.Errorf("timeout waiting for segments to cover end time %s", endTime.Format("15:04:05"))
			}
		}

		// Collect segments that overlap with the requested time range
		for _, segment := range segments {
			segmentStart := segment.Timestamp
			segmentEnd := segmentStart.Add(time.Duration(cm.segmentDuration) * time.Second)

			// Check if the segment overlaps with the requested time range
			if segmentEnd.After(startTime) && segmentStart.Before(endTime) {
				neededSegments = append(neededSegments, segment)
			}
		}

		if len(neededSegments) > 0 {
			// Sort needed segments by timestamp
			sort.Slice(neededSegments, func(i, j int) bool {
				return neededSegments[i].Timestamp.Before(neededSegments[j].Timestamp)
			})

			// Check if we have enough segments to cover the entire range
			firstSegmentStart := neededSegments[0].Timestamp
			lastSegmentEnd := neededSegments[len(neededSegments)-1].Timestamp.Add(time.Duration(cm.segmentDuration) * time.Second)

			if firstSegmentStart.After(startTime) || lastSegmentEnd.Before(endTime) {
				log.Printf("⚠️ Not enough segments to cover full range, waiting for more segments...")
				continue
			}

			// We have enough segments to proceed
			break
		}

		// If we get here, we didn't find any overlapping segments, wait for more
		log.Println("⚠️ No overlapping segments found, waiting for more segments...")
		select {
		case newSegment := <-cm.segmentChan:
			log.Printf("📼 Received new segment: %s", newSegment.Path)
			continue
		case <-time.After(30 * time.Second):
			return fmt.Errorf("timeout waiting for overlapping segments")
		}
	}

	log.Printf("✅ Selected %d segments for clip", len(neededSegments))

	// Create a temporary file list for concat
	concatListPath := filepath.Join(cm.tempDir, "concat_list.txt")
	concatFile, err := os.Create(concatListPath)
	if err != nil {
		return fmt.Errorf("failed to create concat list: %v", err)
	}
	defer os.Remove(concatListPath)

	// Write the file paths to the concat list with correct relative paths
	for _, segment := range neededSegments {
		filename := filepath.Base(segment.Path)
		fmt.Fprintf(concatFile, "file '%s'\n", filename)
	}
	concatFile.Close()

	log.Printf("📝 Created concat list at %s with %d segments", concatListPath, len(neededSegments))

	// Calculate the start offset within the first segment
	firstSegmentStart := neededSegments[0].Timestamp
	startOffset := startTime.Sub(firstSegmentStart).Seconds()
	if startOffset < 0 {
		startOffset = 0
	}

	// Calculate the total duration of the clip
	totalDuration := endTime.Sub(startTime).Seconds()

	// Use FFmpeg to concatenate and trim the clip, outputting as .mp4
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatListPath,
		"-ss", fmt.Sprintf("%.3f", startOffset), // Start offset within the first segment
		"-t", fmt.Sprintf("%.3f", totalDuration), // Total duration of the clip
		"-c:v", "copy",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	}

	log.Printf("🔧 Clip extraction FFmpeg command: ffmpeg %s", strings.Join(args, " "))
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to extract clip from segments: %v\nFFmpeg output: %s", err, stderr.String())
	}

	// Verify the extracted clip exists and has valid content
	extractedDuration, err := cm.verifyClipDuration(outputPath)
	if err != nil {
		// Clean up the invalid file
		os.Remove(outputPath)
		return err
	}

	log.Printf("✅ Successfully extracted clip with duration %.2f seconds", extractedDuration)

	// Check file size as an additional verification
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("could not access the extracted clip file: %v", err)
	}

	if fileInfo.Size() < 1024 {
		os.Remove(outputPath)
		return fmt.Errorf("extracted clip is too small (%.2f KB), possibly no valid data in the segments",
			float64(fileInfo.Size())/1024)
	}

	// Get the aspect ratio of the clip and fix it if necessary
	aspectRatio, err := cm.getVideoAspectRatio(outputPath)
	if err != nil {
		log.Printf("⚠️ Warning: Could not determine aspect ratio of clip: %v", err)
		return nil // Proceed anyway, as this is not critical
	}

	log.Printf("📏 Detected aspect ratio of clip: %s", aspectRatio)

	// Re-encode the clip to explicitly set the aspect ratio
	fixedOutputPath := filepath.Join(cm.tempDir, fmt.Sprintf("fixed_%s", filepath.Base(outputPath)))
	fixArgs := []string{
		"-i", outputPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-aspect", aspectRatio,
		"-y",
		fixedOutputPath,
	}

	log.Printf("🔧 Fixing aspect ratio with FFmpeg command: ffmpeg %s", strings.Join(fixArgs, " "))
	fixCmd := exec.Command("ffmpeg", fixArgs...)
	var fixStderr bytes.Buffer
	fixCmd.Stderr = &fixStderr
	err = fixCmd.Run()
	if err != nil {
		log.Printf("⚠️ Warning: Failed to fix aspect ratio: %v\nFFmpeg output: %s", err, fixStderr.String())
		return nil // Proceed with the original file
	}

	// Replace the original file with the fixed one
	if err := os.Rename(fixedOutputPath, outputPath); err != nil {
		log.Printf("⚠️ Warning: Failed to replace original file with fixed aspect ratio file: %v", err)
		os.Remove(fixedOutputPath)
		return nil // Proceed with the original file
	}

	log.Printf("✅ Aspect ratio fixed for clip: %s", outputPath)

	return nil
}

// verifyClipDuration checks if the extracted clip has a valid duration
func (cm *ClipManager) verifyClipDuration(filePath string) (float64, error) {
	// Use ffprobe to get the duration of the clip
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("verification failed: ffprobe could not analyze clip: %v", err)
	}

	// Parse the duration output
	durationStr := strings.TrimSpace(out.String())
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("verification failed: could not parse clip duration: %v", err)
	}

	// Check if the clip has a reasonable duration
	if duration < 0.5 { // Less than half a second is likely invalid
		return duration, fmt.Errorf("verification failed: clip duration too short (%.2f seconds)", duration)
	}

	return duration, nil
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

// PrepareClipForChatApp prepares a clip for a specific chat app by compressing it if necessary
func (cm *ClipManager) PrepareClipForChatApp(originalFilePath, chatApp string) (string, error) {
	// Define file size limits for each chat app (in MB)
	fileSizeLimits := map[string]float64{
		"discord":    10.0,  // 10 MB limit for Discord
		"telegram":   50.0,  // 50 MB limit for Telegram
		"mattermost": 100.0, // 100 MB limit for Mattermost
	}

	// Compression settings
	const maxCRF = 40     // Maximum CRF for lowest quality
	const initialCRF = 23 // Start CRF for good quality
	const crfStep = 5     // Step size to increase CRF

	// Get the file size limit for the chat app
	targetSizeMB, exists := fileSizeLimits[chatApp]
	if !exists {
		return "", fmt.Errorf("unknown chat app: %s", chatApp)
	}

	// Check original file size
	fileInfo, err := os.Stat(originalFilePath)
	if err != nil {
		return "", fmt.Errorf("could not access the clip file: %v", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	log.Printf("📏 Original file size for %s: %.2f MB (limit: %.2f MB)", chatApp, fileSizeMB, targetSizeMB)

	// If the file is already under the limit, return the original path
	if fileSizeMB <= targetSizeMB {
		log.Printf("✅ File size is under the limit for %s, using original file", chatApp)
		return originalFilePath, nil
	}

	// Get the duration of the clip (for logging)
	duration, err := cm.verifyClipDuration(originalFilePath)
	if err != nil {
		return "", fmt.Errorf("could not verify clip duration: %v", err)
	}
	log.Printf("⏱️ Clip duration for %s: %.2f seconds", chatApp, duration)

	// Get the aspect ratio of the original clip
	aspectRatio, err := cm.getVideoAspectRatio(originalFilePath)
	if err != nil {
		log.Printf("⚠️ Warning: Could not determine aspect ratio for compression: %v", err)
		aspectRatio = "16:9" // Fallback to a common aspect ratio
	}
	log.Printf("📏 Using aspect ratio for compression: %s", chatApp, aspectRatio)

	// Start iterative compression
	crf := initialCRF
	compressedFilePath := filepath.Join(filepath.Dir(originalFilePath), fmt.Sprintf("compressed_%s_%s", chatApp, filepath.Base(originalFilePath)))

	for crf <= maxCRF {
		log.Printf("🔧 Compressing for %s with CRF %d", chatApp, crf)

		// FFmpeg command to compress while preserving aspect ratio
		args := []string{
			"-i", originalFilePath,
			"-vf", "scale='min(1280,iw)':-2", // Scale width to max 1280, preserve aspect ratio
			"-c:v", "libx264",                // Video codec
			"-crf", strconv.Itoa(crf),        // Constant Rate Factor
			"-preset", "medium",              // Balance speed vs compression
			"-c:a", "aac",                    // Audio codec
			"-b:a", "96k",                    // Audio bitrate
			"-movflags", "+faststart",        // Optimize for streaming
			"-aspect", aspectRatio,           // Explicitly set the aspect ratio
			"-y",                             // Overwrite output without asking
			compressedFilePath,
		}

		log.Printf("🔧 Compression command for %s: ffmpeg %s", chatApp, strings.Join(args, " "))
		cmd := exec.Command("ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			log.Printf("❌ Compression failed for %s: %v\nFFmpeg output: %s", chatApp, err, stderr.String())
			return originalFilePath, fmt.Errorf("compression failed: %v", err)
		}

		// Check size of compressed file
		compressedInfo, err := os.Stat(compressedFilePath)
		if err != nil {
			log.Printf("❌ Error checking compressed file for %s: %v, falling back to original", chatApp, err)
			return originalFilePath, fmt.Errorf("could not access compressed file: %v", err)
		}

		compressedSizeMB := float64(compressedInfo.Size()) / 1024 / 1024
		log.Printf("📏 Compressed file size for %s: %.2f MB", chatApp, compressedSizeMB)

		if compressedSizeMB <= targetSizeMB {
			log.Printf("✅ Compression succeeded for %s with CRF %d", chatApp, crf)
			return compressedFilePath, nil
		}

		// Increase CRF for the next iteration
		crf += crfStep
	}

	log.Printf("❌ Could not compress file under %.2f MB for %s, even with CRF %d", targetSizeMB, chatApp, maxCRF)
	return compressedFilePath, fmt.Errorf("file size still exceeds %.2f MB for %s after maximum compression", targetSizeMB, chatApp)
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
	cm.logger.Error("Error sending clip to %s: %v", serviceName, err)

	// Retry logic
	for attempt := 1; attempt <= cm.maxRetries; attempt++ {
		cm.logger.Warning("Retry %d/%d for %s...", attempt, cm.maxRetries, serviceName)

		// Wait before retrying
		time.Sleep(cm.retryDelay)

		// Try again
		err = operation()
		if err == nil {
			cm.logger.Success("Retry %d/%d for %s succeeded", attempt, cm.maxRetries, serviceName)
			return nil
		}

		cm.logger.Error("Retry %d/%d for %s failed: %v", attempt, cm.maxRetries, serviceName, err)
	}

	// All retries failed
	cm.logger.Error("All %d retries failed for %s", cm.maxRetries, serviceName)
	return fmt.Errorf("failed to send clip to %s after %d attempts: %v", serviceName, cm.maxRetries+1, err)
}

// sendToTelegram sends a clip to Telegram
func (cm *ClipManager) sendToTelegram(filePath, botToken, chatID string, category string, poolManagerData *PoolManagerData) error {
	// Define the operation to be retried
	operation := func() error {
		file, err := os.Open(filePath)
		if err != nil {
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
		// Add note about aspect ratio distortion
		captionText += "\n(if distorted, download and view elsewhere)"

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

		// Add the caption field with the note
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
func (cm *ClipManager) SendToChatApp(originalFilePath string, req ClipRequest) error {
	var poolManagerData *PoolManagerData = nil

	// Split the chat_app string into a list of chat apps
	chatApps := strings.Split(strings.ToLower(req.ChatApp), ",")

	var wg sync.WaitGroup
	errors := make(chan error, len(chatApps))
	// Keep track of compressed files to clean up later
	compressedFiles := make(map[string]string)

	for _, app := range chatApps {
		app = strings.TrimSpace(app)

		// Prepare the clip for this specific chat app
		filePath, err := cm.PrepareClipForChatApp(originalFilePath, app)
		if err != nil {
			cm.logger.Error("Error preparing clip for %s: %v", app, err)
			errors <- fmt.Errorf("error preparing clip for %s: %v", app, err)
			continue
		}

		// If a compressed file was created, store it for cleanup
		if filePath != originalFilePath {
			compressedFiles[app] = filePath
		}

		wg.Add(1)
		go func(app, filePath string) {
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
				err = fmt.Errorf("unsupported chat app: %s", app)
			}

			if err != nil {
				cm.logger.Error("Error sending clip to %s: %v", app, err)
				errors <- fmt.Errorf("error sending to %s: %v", app, err)
			} else {
				cm.logger.Success("Successfully sent clip to %s", app)
			}
		}(app, filePath)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Clean up compressed files
	for app, filePath := range compressedFiles {
		cm.logger.Info("Cleaning up compressed file for %s: %s", app, filePath)
		os.Remove(filePath)
	}

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
	return time.Now().Format("2006-01-02")
}

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
	containerPort := "5000"

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

// getHostPort determines the external port that users should connect to
func getHostPort() string {
	hostPort := os.Getenv("HOST_PORT")
	if hostPort == "" {
		return "5001" // Default to 5001 if not specified
	}
	return hostPort
}