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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

type ClipRequest struct {
	CameraIP          string `json:"camera_ip"`
	BacktrackSeconds  int    `json:"backtrack_seconds"`
	DurationSeconds   int    `json:"duration_seconds"`
	ChatApps          string `json:"chat_app"`
	Category          string `json:"category"`
	Team1             string `json:"team1"`
	Team2             string `json:"team2"`
	AdditionalText    string `json:"additional_text"`
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramChatID    string `json:"telegram_chat_id"`
	MattermostURL     string `json:"mattermost_url"`
	MattermostToken   string `json:"mattermost_token"`
	MattermostChannel string `json:"mattermost_channel"`
	DiscordWebhookURL string `json:"discord_webhook_url"`
}

type ClipResponse struct {
	Message string `json:"message"`
}

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
	segmentPattern  string
	recording       bool
	segments        []SegmentInfo
	segmentsMutex   sync.RWMutex
	segmentChan     chan SegmentInfo
	segmentDuration int
	log             *Logger
}

func NewClipManager(tempDir, hostPort, cameraIP string) (*ClipManager, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory %s: %v", tempDir, err)
	}
	absTemp, err := filepath.Abs(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %v", tempDir, err)
	}
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
		segmentChan:     make(chan SegmentInfo, 100),
		segmentDuration: 5,
		log:             NewLogger(),
	}, nil
}

func (cm *ClipManager) HandleClipRequest(w http.ResponseWriter, r *http.Request) {
	cm.limiter.Wait(r.Context())

	var req ClipRequest
	if r.Method == http.MethodGet {
		req.CameraIP = r.URL.Query().Get("camera_ip")
		req.BacktrackSeconds, _ = strconv.Atoi(r.URL.Query().Get("backtrack_seconds"))
		req.DurationSeconds, _ = strconv.Atoi(r.URL.Query().Get("duration_seconds"))
		req.ChatApps = strings.ToLower(r.URL.Query().Get("chat_app"))
		req.Category = r.URL.Query().Get("category")
		req.Team1 = r.URL.Query().Get("team1")
		req.Team2 = r.URL.Query().Get("team2")
		req.AdditionalText = r.URL.Query().Get("additional_text")
		req.TelegramBotToken = r.URL.Query().Get("telegram_bot_token")
		req.TelegramChatID = r.URL.Query().Get("telegram_chat_id")
		req.MattermostURL = r.URL.Query().Get("mattermost_url")
		req.MattermostToken = r.URL.Query().Get("mattermost_token")
		req.MattermostChannel = r.URL.Query().Get("mattermost_channel")
		req.DiscordWebhookURL = r.URL.Query().Get("discord_webhook_url")
	} else if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := cm.validateRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go func() {
		fileName := fmt.Sprintf("clip_%d.mp4", time.Now().Unix())
		outputPath := filepath.Join(cm.tempDir, fileName)
		if err := cm.RecordClip(req.BacktrackSeconds, req.DurationSeconds, outputPath, time.Now()); err != nil {
			cm.log.Error("Failed to record clip: %v", err)
			return
		}
		cm.PrepareClipForChatApp(outputPath, req)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ClipResponse{Message: "Clip recording and sending started"})
}

func (cm *ClipManager) validateRequest(req ClipRequest) error {
	if req.CameraIP == "" {
		req.CameraIP = cm.cameraIP
	}
	if req.CameraIP == "" {
		return fmt.Errorf("camera_ip is required")
	}
	if req.BacktrackSeconds < 0 || req.BacktrackSeconds > 300 {
		return fmt.Errorf("backtrack_seconds must be between 0 and 300")
	}
	if req.DurationSeconds <= 0 || req.DurationSeconds > 300 {
		return fmt.Errorf("duration_seconds must be between 1 and 300")
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

func (cm *ClipManager) addSegment(segmentPath string) {
	cm.segmentsMutex.Lock()
	defer cm.segmentsMutex.Unlock()

	absolutePath := filepath.Join(cm.tempDir, segmentPath)
	segmentInfo := SegmentInfo{
		Path:      absolutePath,
		Timestamp: time.Now(),
	}
	cm.segments = append(cm.segments, segmentInfo)

	sort.Slice(cm.segments, func(i, j int) bool {
		return cm.segments[i].Timestamp.Before(cm.segments[j].Timestamp)
	})

	const maxSegments = 62
	if len(cm.segments) > maxSegments {
		for _, old := range cm.segments[:len(cm.segments)-maxSegments] {
			if err := os.Remove(old.Path); err != nil {
				cm.log.Error("Failed to remove old segment %s: %v", old.Path, err)
			} else {
				cm.log.Info("Removed old segment: %s", old.Path)
			}
		}
		cm.segments = cm.segments[len(cm.segments)-maxSegments:]
	}

	cm.segmentChan <- segmentInfo
	cm.log.Info("Added segment: %s, total: %d (up to %d seconds)", segmentPath, len(cm.segments), len(cm.segments)*cm.segmentDuration)
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

func (cm *ClipManager) PrepareClipForChatApp(filePath string, req ClipRequest) {
	chatApps := strings.Split(req.ChatApps, ",")
	for _, app := range chatApps {
		app = strings.TrimSpace(app)
		switch app {
		case "telegram":
			if err := cm.sendToTelegram(filePath, req.TelegramBotToken, req.TelegramChatID, req); err != nil {
				cm.log.Error("Failed to send to Telegram: %v", err)
			}
		case "mattermost":
			if err := cm.sendToMattermost(filePath, req.MattermostURL, req.MattermostToken, req.MattermostChannel, req); err != nil {
				cm.log.Error("Failed to send to Mattermost: %v", err)
			}
		case "discord":
			if err := cm.sendToDiscord(filePath, req.DiscordWebhookURL, req); err != nil {
				cm.log.Error("Failed to send to Discord: %v", err)
			}
		}
	}
}

func (cm *ClipManager) sendToTelegram(filePath, botToken, chatID string, req ClipRequest) error {
	operation := func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("could not open file for Telegram: %v", err)
		}
		defer file.Close()

		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		if err := writer.WriteField("chat_id", chatID); err != nil {
			return fmt.Errorf("error adding chat_id to Telegram request: %v", err)
		}
		captionText := cm.buildClipMessage(req)
		if err := writer.WriteField("caption", captionText); err != nil {
			return fmt.Errorf("error adding caption to Telegram request: %v", err)
		}
		part, err := writer.CreateFormFile("video", filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("error creating form file for Telegram: %v", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return fmt.Errorf("error copying file to Telegram form: %v", err)
		}
		writer.Close()

		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendVideo", botToken)
		resp, err := cm.httpClient.Post(url, writer.FormDataContentType(), &b)
		if err != nil {
			return fmt.Errorf("Telegram API request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Telegram API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
		}
		cm.log.Info("Successfully sent clip to Telegram")
		return nil
	}
	return cm.RetryOperation(operation, "Telegram")
}

func (cm *ClipManager) sendToMattermost(filePath, serverURL, token, channel string, req ClipRequest) error {
	operation := func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("could not open file for Mattermost: %v", err)
		}
		defer file.Close()

		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, err := writer.CreateFormFile("files", filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("error creating form file for Mattermost: %v", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return fmt.Errorf("error copying file to Mattermost form: %v", err)
		}
		if err := writer.WriteField("channel_id", channel); err != nil {
			return fmt.Errorf("error adding channel_id to Mattermost request: %v", err)
		}
		writer.Close()

		url := fmt.Sprintf("%s/api/v4/files", serverURL)
		req, err := http.NewRequest("POST", url, &b)
		if err != nil {
			return fmt.Errorf("error creating Mattermost file request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := cm.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("Mattermost file upload failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Mattermost file upload returned non-201 status: %d, body: %s", resp.StatusCode, string(body))
		}

		var fileResp struct {
			FileInfos []struct {
				ID string `json:"id"`
			} `json:"file_infos"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
			return fmt.Errorf("error decoding Mattermost file response: %v", err)
		}
		if len(fileResp.FileInfos) == 0 {
			return fmt.Errorf("no file ID returned from Mattermost")
		}
		fileID := fileResp.FileInfos[0].ID

		post := struct {
			ChannelID string   `json:"channel_id"`
			Message   string   `json:"message"`
			FileIDs   []string `json:"file_ids"`
		}{
			ChannelID: channel,
			Message:   cm.buildClipMessage(req),
			FileIDs:   []string{fileID},
		}
		postData, _ := json.Marshal(post)
		postReq, err := http.NewRequest("POST", serverURL+"/api/v4/posts", bytes.NewBuffer(postData))
		if err != nil {
			return fmt.Errorf("error creating Mattermost post request: %v", err)
		}
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+token)

		postResp, err := cm.httpClient.Do(postReq)
		if err != nil {
			return fmt.Errorf("Mattermost post request failed: %v", err)
		}
		defer postResp.Body.Close()

		if postResp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(postResp.Body)
			return fmt.Errorf("Mattermost post returned non-201 status: %d, body: %s", postResp.StatusCode, string(body))
		}
		cm.log.Info("Successfully sent clip to Mattermost")
		return nil
	}
	return cm.RetryOperation(operation, "Mattermost")
}

func (cm *ClipManager) sendToDiscord(filePath, webhookURL string, req ClipRequest) error {
	operation := func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("could not open file for Discord: %v", err)
		}
		defer file.Close()

		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			return fmt.Errorf("error creating form file for Discord: %v", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			return fmt.Errorf("error copying file to Discord form: %v", err)
		}
		if err := writer.WriteField("content", cm.buildClipMessage(req)); err != nil {
			return fmt.Errorf("error adding content to Discord request: %v", err)
		}
		writer.Close()

		resp, err := cm.httpClient.Post(webhookURL, writer.FormDataContentType(), &b)
		if err != nil {
			return fmt.Errorf("Discord webhook request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Discord webhook returned non-200/204 status: %d, body: %s", resp.StatusCode, string(body))
		}
		cm.log.Info("Successfully sent clip to Discord")
		return nil
	}
	return cm.RetryOperation(operation, "Discord")
}

func (cm *ClipManager) RetryOperation(operation func() error, appName string) error {
	for i := 0; i < cm.maxRetries; i++ {
		if err := operation(); err != nil {
			cm.log.Warn("Attempt %d failed for %s: %v", i+1, appName, err)
			if i == cm.maxRetries-1 {
				return err
			}
			time.Sleep(cm.retryDelay)
			continue
		}
		return nil
	}
	return nil
}

func (cm *ClipManager) buildClipMessage(req ClipRequest) string {
	base := fmt.Sprintf("New %sClip: %s", optionalCategory(req.Category), cm.formatDate())

	var teams string
	if req.Team1 != "" && req.Team2 != "" {
		teams = fmt.Sprintf(" / %s vs %s", req.Team1, req.Team2)
	}

	var extra string
	if req.AdditionalText != "" {
		extra = fmt.Sprintf(" - %s", req.AdditionalText)
	}

	return base + teams + extra
}

func optionalCategory(category string) string {
	if category != "" {
		return category + " "
	}
	return ""
}

func (cm *ClipManager) formatDate() string {
	return time.Now().Format("2006-01-02")
}

type Logger struct {
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Info(format string, v ...interface{}) {
	log.Printf("\033[34mℹ️ %s\033[0m", fmt.Sprintf(format, v...))
}

func (l *Logger) Error(format string, v ...interface{}) {
	log.Printf("\033[31m❌ %s\033[0m", fmt.Sprintf(format, v...))
}

func (l *Logger) Warn(format string, v ...interface{}) {
	log.Printf("\033[33m⚠️ %s\033[0m", fmt.Sprintf(format, v...))
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	tempDir := os.Getenv("TEMP_DIR")
	if tempDir == "" {
		tempDir = "./clips"
	}
	hostPort := os.Getenv("HOST_PORT")
	if hostPort == "" {
		hostPort = "5001"
	}
	cameraIP := os.Getenv("CAMERA_IP")

	cm, err := NewClipManager(tempDir, hostPort, cameraIP)
	if err != nil {
		log.Fatalf("Failed to initialize ClipManager: %v", err)
	}

	go cm.StartBackgroundRecording()

	http.HandleFunc("/api/clip", cm.HandleClipRequest)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	cm.log.Info("Starting server on port %s", hostPort)
	if err := http.ListenAndServe(":"+hostPort, nil); err != nil {
		cm.log.Error("Server failed: %v", err)
	}
}