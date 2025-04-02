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

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
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

func NewClipManager(tempDir string, hostPort string, cameraIP string) (*ClipManager, error) {
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

func (cm *ClipManager) RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cm.limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			cm.log.Error("Rate limit exceeded for IP: %s", r.RemoteAddr)
			return
		}
		next(w, r)
	}
}

func (cm *ClipManager) HandleClipRequest(w http.ResponseWriter, r *http.Request) {
    startTime := time.Now()
    requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

    if r.Method != http.MethodGet && r.Method != http.MethodPost {
        http.Error(w, "Method not allowed, use GET or POST", http.StatusMethodNotAllowed)
        return
    }

    fileName := fmt.Sprintf("clip_%d.mp4", time.Now().Unix())
    filePath := filepath.Join(cm.tempDir, fileName)

    response := ClipResponse{Message: "Clip recording and sending started"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)

    go func() {
        defer func() {
            processingTime := time.Since(startTime)
            cm.log.Info("[%s] Total processing time: %v", requestID, processingTime)
        }()

        backtrackSeconds, _ := strconv.Atoi(r.URL.Query().Get("backtrack_seconds"))
        durationSeconds, _ := strconv.Atoi(r.URL.Query().Get("duration_seconds"))

        cm.log.Info("[%s] Extracting clip for backtrack: %d seconds, duration: %d seconds",
            requestID, backtrackSeconds, durationSeconds)
        err := cm.RecordClip(backtrackSeconds, durationSeconds, filePath, startTime)
        if err != nil {
            cm.log.Error("[%s] Recording error: %v", requestID, err)
            return
        }
        cm.log.Success("[%s] Clip recording completed", requestID)

        if err := cm.SendToChatApp(filePath, r); err != nil {
            cm.log.Error("[%s] Error sending clip: %v", requestID, err)
        }

        os.Remove(filePath)
    }()
}

func (cm *ClipManager) validateRequest(req *ClipRequest) error {
	req.CameraIP = cm.cameraIP

	if req.ChatApps == "" {
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

	chatApps := strings.Split(strings.ToLower(req.ChatApps), ",")

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

func (cm *ClipManager) StartBackgroundRecording() {
	if cm.recording {
		cm.log.Warning("Background recording is already running")
		return
	}

	cm.recording = true

	cm.log.Info("Starting background recording with segments for backtracking capability...")

	go func() {
		attempt := 1
		cycle := 0

		for {
			availableSpace, err := cm.CheckDiskSpace()
			if err != nil {
				cm.log.Error("Error checking disk space: %v, continuing with recording", err)
			} else {
				availableSpaceMB := availableSpace / (1024 * 1024)
				cm.log.Info("Available disk space: %d MB", availableSpaceMB)

				if availableSpaceMB < 500 {
					cm.log.Warning("Low disk space (< 500MB), skipping recording cycle, retrying in 30 seconds...")
					time.Sleep(30 * time.Second)
					continue
				}
			}

			segmentPattern := fmt.Sprintf("%s_cycle%d_%%03d.ts", strings.TrimSuffix(cm.segmentPattern, "_%03d.ts"), cycle)
			segmentList := filepath.Join(cm.tempDir, fmt.Sprintf("segments_cycle%d.m3u8", cycle))

			args := []string{
				"-rtsp_transport", "tcp",
				"-i", cm.cameraIP,
				"-f", "segment",
				"-segment_time", "5",
				"-segment_format", "mpegts",
				"-reset_timestamps", "1",
				"-segment_list", segmentList,
				"-segment_list_type", "m3u8",
				"-c:v", "copy",
				"-c:a", "copy",
				"-y",
				segmentPattern,
			}

			logCmd := fmt.Sprintf("ffmpeg %s", strings.Join(args, " "))
			cm.log.Debug("Segment recording FFmpeg command: %s", logCmd)

			cmd := exec.Command("ffmpeg", args...)

			stderr, err := cmd.StderrPipe()
			if err != nil {
				cm.log.Error("Error getting stderr pipe: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if err := cmd.Start(); err != nil {
				cm.log.Error("Error starting FFmpeg: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			go func(cycle int) {
				scanner := bufio.NewScanner(stderr)
				segmentRegex := regexp.MustCompile(fmt.Sprintf(`Opening '.*/(segment_cycle%d_\d+\.ts)' for writing`, cycle))

				for scanner.Scan() {
					line := scanner.Text()
					matches := segmentRegex.FindStringSubmatch(line)
					if len(matches) > 1 {
						segmentFile := matches[1]
						cm.log.Success("New segment created: %s", segmentFile)
						cm.addSegment(segmentFile)
					}
				}

				if err := scanner.Err(); err != nil {
					cm.log.Error("Error reading FFmpeg stderr: %v", err)
				}
			}(cycle)

			err = cmd.Wait()

			if err != nil {
				stderrBytes, _ := io.ReadAll(stderr)
				errMsg := string(stderrBytes)
				cm.log.Error("FFmpeg error: %v\nFFmpeg output: %s", err, errMsg)
				if isConnectionError(errMsg) {
					cm.log.Warning("Camera disconnected, retrying connection (attempt %d)...", attempt)
					attempt++
					time.Sleep(10 * time.Second)
					continue
				}

				cm.log.Error("Background recording error: %v", err)
				time.Sleep(5 * time.Second)
				attempt++
				continue
			}

			cm.log.Info("Background recording cycle completed, starting next cycle...")
			attempt = 1
			cycle++
		}
	}()
}

func (cm *ClipManager) CheckDiskSpace() (uint64, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(cm.tempDir, &stat)
	if err != nil {
		return 0, fmt.Errorf("failed to get filesystem stats: %v", err)
	}

	availableSpace := stat.Bavail * uint64(stat.Bsize)
	return availableSpace, nil
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

func (cm *ClipManager) RecordClip(backtrackSeconds, durationSeconds int, outputPath string, requestTime time.Time) error {
	startTime := requestTime.Add(-time.Duration(backtrackSeconds) * time.Second)
	endTime := startTime.Add(time.Duration(durationSeconds) * time.Second)

	cm.log.Info("📹 Requested clip from %s to %s", startTime.Format("15:04:05"), endTime.Format("15:04:05"))

	var neededSegments []SegmentInfo

	for {
		cm.segmentsMutex.RLock()
		segments := make([]SegmentInfo, len(cm.segments))
		copy(segments, cm.segments)
		cm.segmentsMutex.RUnlock()

		if len(segments) == 0 {
			cm.log.Warning("⚠️ No segments available, waiting for first segment...")
			select {
			case newSegment := <-cm.segmentChan:
				cm.log.Info("📼 Received first segment: %s", newSegment.Path)
				continue
			case <-time.After(30 * time.Second):
				return fmt.Errorf("timeout waiting for first segment")
			}
		}

		neededSegments = []SegmentInfo{}
		earliestTime := segments[0].Timestamp
		latestTime := segments[len(segments)-1].Timestamp

		if startTime.Before(earliestTime) {
			cm.log.Warning("⚠️ Requested start time %s is before earliest segment at %s", startTime.Format("15:04:05"), earliestTime.Format("15:04:05"))
			startTime = earliestTime
			endTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
			cm.log.Info("🔄 Adjusted clip time to %s to %s", startTime.Format("15:04:05"), endTime.Format("15:04:05"))
		}

		if endTime.After(latestTime) {
			cm.log.Info("⏳ End time %s is after latest segment at %s, waiting for more segments...", endTime.Format("15:04:05"), latestTime.Format("15:04:05"))
			timeout := time.After(2 * time.Duration(durationSeconds) * time.Second)
			select {
			case newSegment := <-cm.segmentChan:
				cm.log.Info("📼 Received new segment: %s at %s", newSegment.Path, newSegment.Timestamp.Format("15:04:05"))
				continue
			case <-timeout:
				return fmt.Errorf("timeout waiting for segments to cover end time %s", endTime.Format("15:04:05"))
			}
		}

		for _, segment := range segments {
			segmentStart := segment.Timestamp
			segmentEnd := segmentStart.Add(time.Duration(cm.segmentDuration) * time.Second)

			if segmentEnd.After(startTime) && segmentStart.Before(endTime) {
				neededSegments = append(neededSegments, segment)
			}
		}

		if len(neededSegments) > 0 {
			sort.Slice(neededSegments, func(i, j int) bool {
				return neededSegments[i].Timestamp.Before(neededSegments[j].Timestamp)
			})

			firstSegmentStart := neededSegments[0].Timestamp
			lastSegmentEnd := neededSegments[len(neededSegments)-1].Timestamp.Add(time.Duration(cm.segmentDuration) * time.Second)

			if firstSegmentStart.After(startTime) || lastSegmentEnd.Before(endTime) {
				cm.log.Warning("Not enough segments to cover full range, waiting for more segments...")
				continue
			}

			break
		}

		cm.log.Warning("No overlapping segments found, waiting for more segments...")
		select {
		case newSegment := <-cm.segmentChan:
			cm.log.Info("📼 Received new segment: %s", newSegment.Path)
			continue
		case <-time.After(30 * time.Second):
			return fmt.Errorf("timeout waiting for overlapping segments")
		}
	}

	cm.log.Success("✅ Selected %d segments for clip", len(neededSegments))

	concatListPath := filepath.Join(cm.tempDir, "concat_list.txt")
	concatFile, err := os.Create(concatListPath)
	if err != nil {
		return fmt.Errorf("failed to create concat list: %v", err)
	}
	defer os.Remove(concatListPath)

	for _, segment := range neededSegments {
		filename := filepath.Base(segment.Path)
		fmt.Fprintf(concatFile, "file '%s'\n", filename)
	}
	concatFile.Close()

	cm.log.Info("📝 Created concat list at %s with %d segments", concatListPath, len(neededSegments))

	firstSegmentStart := neededSegments[0].Timestamp
	startOffset := startTime.Sub(firstSegmentStart).Seconds()
	if startOffset < 0 {
		startOffset = 0
	}

	totalDuration := endTime.Sub(startTime).Seconds()

	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatListPath,
		"-ss", fmt.Sprintf("%.3f", startOffset),
		"-t", fmt.Sprintf("%.3f", totalDuration),
		"-c:v", "copy",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	}

	cm.log.Debug("🔧 Clip extraction FFmpeg command: ffmpeg %s", strings.Join(args, " "))
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to extract clip from segments: %v\nFFmpeg output: %s", err, stderr.String())
	}

	extractedDuration, err := cm.verifyClipDuration(outputPath)
	if err != nil {
		os.Remove(outputPath)
		return err
	}

	cm.log.Success("✅ Successfully extracted clip with duration %.2f seconds", extractedDuration)

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("could not access the extracted clip file: %v", err)
	}

	if fileInfo.Size() < 1024 {
		os.Remove(outputPath)
		return fmt.Errorf("extracted clip is too small (%.2f KB), possibly no valid data in the segments", float64(fileInfo.Size())/1024)
	}

	aspectRatio, err := cm.getVideoAspectRatio(outputPath)
	if err != nil {
		cm.log.Warning("⚠️ Warning: Could not determine aspect ratio of clip: %v", err)
		return nil
	}

	cm.log.Info("📏 Detected aspect ratio of clip: %s", aspectRatio)

	fixedOutputPath := filepath.Join(cm.tempDir, fmt.Sprintf("fixed_%s", filepath.Base(outputPath)))
	fixArgs := []string{
		"-i", outputPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-aspect", aspectRatio,
		"-y",
		fixedOutputPath,
	}

	cm.log.Debug("🔧 Fixing aspect ratio with FFmpeg command: ffmpeg %s", strings.Join(fixArgs, " "))
	fixCmd := exec.Command("ffmpeg", fixArgs...)
	var fixStderr bytes.Buffer
	fixCmd.Stderr = &fixStderr
	err = fixCmd.Run()
	if err != nil {
		cm.log.Warning("⚠️ Warning: Failed to fix aspect ratio: %v\nFFmpeg output: %s", err, fixStderr.String())
		return nil
	}

	if err := os.Rename(fixedOutputPath, outputPath); err != nil {
		cm.log.Warning("⚠️ Warning: Failed to replace original file with fixed aspect ratio file: %v", err)
		os.Remove(fixedOutputPath)
		return nil
	}

	cm.log.Success("✅ Aspect ratio fixed for clip: %s", outputPath)
	return nil
}

func (cm *ClipManager) verifyClipDuration(filePath string) (float64, error) {
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

	durationStr := strings.TrimSpace(out.String())
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("verification failed: could not parse clip duration: %v", err)
	}

	if duration < 0.5 {
		return duration, fmt.Errorf("verification failed: clip duration too short (%.2f seconds)", duration)
	}

	return duration, nil
}

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

func (cm *ClipManager) PrepareClipForChatApp(originalFilePath, chatApp string) (string, error) {
	fileSizeLimits := map[string]float64{
		"discord":    10.0,
		"telegram":   50.0,
		"mattermost": 100.0,
	}

	const maxCRF = 40
	const initialCRF = 23
	const crfStep = 5

	targetSizeMB, exists := fileSizeLimits[chatApp]
	if !exists {
		return "", fmt.Errorf("unknown chat app: %s", chatApp)
	}

	fileInfo, err := os.Stat(originalFilePath)
	if err != nil {
		return "", fmt.Errorf("could not access the clip file: %v", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	cm.log.Info("📏 Original file size for %s: %.2f MB (limit: %.2f MB)", chatApp, fileSizeMB, targetSizeMB)

	if fileSizeMB <= targetSizeMB {
		cm.log.Success("✅ File size is under the limit for %s, using original file", chatApp)
		return originalFilePath, nil
	}

	duration, err := cm.verifyClipDuration(originalFilePath)
	if err != nil {
		return "", fmt.Errorf("could not verify clip duration: %v", err)
	}
	cm.log.Info("⏱️ Clip duration for %s: %.2f seconds", chatApp, duration)

	aspectRatio, err := cm.getVideoAspectRatio(originalFilePath)
	if err != nil {
		cm.log.Warning("⚠️ Warning: Could not determine aspect ratio for compression: %v", err)
		aspectRatio = "16:9"
	}
	cm.log.Info("📏 Using aspect ratio for compression: %s", aspectRatio)

	crf := initialCRF
	compressedFilePath := filepath.Join(filepath.Dir(originalFilePath), fmt.Sprintf("compressed_%s_%s", chatApp, filepath.Base(originalFilePath)))

	for crf <= maxCRF {
		cm.log.Info("🔧 Compressing for %s with CRF %d", chatApp, crf)

		args := []string{
			"-i", originalFilePath,
			"-vf", "scale='min(1280,iw)':-2",
			"-c:v", "libx264",
			"-crf", strconv.Itoa(crf),
			"-preset", "medium",
			"-c:a", "aac",
			"-b:a", "96k",
			"-movflags", "+faststart",
			"-aspect", aspectRatio,
			"-y",
			compressedFilePath,
		}

		cm.log.Debug("🔧 Compression command for %s: ffmpeg %s", chatApp, strings.Join(args, " "))
		cmd := exec.Command("ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			cm.log.Error("❌ Compression failed for %s: %v\nFFmpeg output: %s", chatApp, err, stderr.String())
			return originalFilePath, fmt.Errorf("compression failed: %v", err)
		}

		compressedInfo, err := os.Stat(compressedFilePath)
		if err != nil {
			cm.log.Error("❌ Error checking compressed file for %s: %v, falling back to original", chatApp, err)
			return originalFilePath, fmt.Errorf("could not access compressed file: %v", err)
		}

		compressedSizeMB := float64(compressedInfo.Size()) / 1024 / 1024
		cm.log.Info("📏 Compressed file size for %s: %.2f MB", chatApp, compressedSizeMB)

		if compressedSizeMB <= targetSizeMB {
			cm.log.Success("✅ Compression succeeded for %s with CRF %d", chatApp, crf)
			return compressedFilePath, nil
		}

		crf += crfStep
	}

	cm.log.Error("❌ Could not compress file under %.2f MB for %s, even with CRF %d", targetSizeMB, chatApp, maxCRF)
	return compressedFilePath, fmt.Errorf("file size still exceeds %.2f MB for %s after maximum compression", targetSizeMB, chatApp)
}

func (cm *ClipManager) RetryOperation(operation func() error, serviceName string) error {
	var err error

	err = operation()
	if err == nil {
		return nil
	}

	cm.log.Error("Error sending clip to %s: %v", serviceName, err)

	for attempt := 1; attempt <= cm.maxRetries; attempt++ {
		cm.log.Warning("Retry %d/%d for %s...", attempt, cm.maxRetries, serviceName)
		time.Sleep(cm.retryDelay)

		err = operation()
		if err == nil {
			cm.log.Success("Retry %d/%d for %s succeeded", attempt, cm.maxRetries, serviceName)
			return nil
		}

		cm.log.Error("Retry %d/%d for %s failed: %v", attempt, cm.maxRetries, serviceName, err)
	}

	cm.log.Error("All %d retries failed for %s", cm.maxRetries, serviceName)
	return fmt.Errorf("failed to send clip to %s after %d attempts: %v", serviceName, cm.maxRetries+1, err)
}

func (cm *ClipManager) sendToTelegram(filePath, botToken, chatID string, r *http.Request) error {
    operation := func() error {
        file, err := os.Open(filePath)
        if err != nil {
            return fmt.Errorf("could not open file for sending to Telegram: %v", err)
        }
        defer file.Close()

        captionText := cm.buildClipMessage(r)
        captionText += "\n(if distorted, download and view elsewhere)"

        chatID = strings.Trim(chatID, `"'`)
        if chatID == "" {
            return fmt.Errorf("error: telegram_chat_id is empty, cannot send to Telegram")
        }

        reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendVideo", botToken)

        cm.log.Info("Sending clip to Telegram. File: %s", filepath.Base(filePath))

        var requestBody bytes.Buffer
        writer := multipart.NewWriter(&requestBody)

        if err := writer.WriteField("chat_id", chatID); err != nil {
            return fmt.Errorf("error preparing Telegram request: %v", err)
        }

        if err := writer.WriteField("caption", captionText); err != nil {
            return fmt.Errorf("error adding caption to Telegram request: %v", err)
        }

        part, err := writer.CreateFormFile("video", filepath.Base(filePath))
        if err != nil {
            return fmt.Errorf("error creating file field for Telegram: %v", err)
        }

        if _, err := io.Copy(part, file); err != nil {
            return fmt.Errorf("error copying file to Telegram request: %v", err)
        }

        if err := writer.Close(); err != nil {
            return fmt.Errorf("error finalizing Telegram request: %v", err)
        }

        req, err := http.NewRequest("POST", reqURL, &requestBody)
        if err != nil {
            return fmt.Errorf("error creating Telegram request: %v", err)
        }

        req.Header.Set("Content-Type", writer.FormDataContentType())

        resp, err := cm.httpClient.Do(req)
        if err != nil {
            return fmt.Errorf("error sending clip to Telegram: %v", err)
        }
        defer resp.Body.Close()

        bodyBytes, _ := io.ReadAll(resp.Body)
        responseBody := string(bodyBytes)

        if resp.StatusCode != http.StatusOK {
            return fmt.Errorf("telegram API error: %s - %s", resp.Status, responseBody)
        }

        cm.log.Success("Clip successfully sent to Telegram")
        return nil
    }

    return cm.RetryOperation(operation, "Telegram")
}

func (cm *ClipManager) sendToMattermost(filePath, mattermostURL, token, channelID string, r *http.Request) error {
    operation := func() error {
        file, err := os.Open(filePath)
        if err != nil {
            return fmt.Errorf("could not open file for sending to Mattermost: %v", err)
        }
        defer file.Close()

        var requestBody bytes.Buffer
        writer := multipart.NewWriter(&requestBody)

        if err := writer.WriteField("channel_id", channelID); err != nil {
            return fmt.Errorf("error preparing Mattermost request: %v", err)
        }

        part, err := writer.CreateFormFile("files", filepath.Base(filePath))
        if err != nil {
            return fmt.Errorf("error creating file field for Mattermost: %v", err)
        }

        if _, err := io.Copy(part, file); err != nil {
            return fmt.Errorf("error copying file to Mattermost request: %v", err)
        }

        if err := writer.Close(); err != nil {
            return fmt.Errorf("error finalizing Mattermost request: %v", err)
        }

        fileUploadURL := fmt.Sprintf("%s/api/v4/files", mattermostURL)
        cm.log.Info("Uploading file to Mattermost")

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

        if resp.StatusCode >= 300 {
            bodyBytes, _ := io.ReadAll(resp.Body)
            return fmt.Errorf("mattermost file upload error: %s - %s", resp.Status, string(bodyBytes))
        }

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

        messageText := cm.buildClipMessage(r)

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

        cm.log.Success("Clip successfully sent to Mattermost")
        return nil
    }

    return cm.RetryOperation(operation, "Mattermost")
}

func (cm *ClipManager) sendToDiscord(filePath, webhookURL string, r *http.Request) error {
    operation := func() error {
        file, err := os.Open(filePath)
        if err != nil {
            return fmt.Errorf("could not open file for sending to Discord: %v", err)
        }
        defer file.Close()

        messageText := cm.buildClipMessage(r)

        var requestBody bytes.Buffer
        writer := multipart.NewWriter(&requestBody)

        if err := writer.WriteField("content", messageText); err != nil {
            return fmt.Errorf("error adding content to Discord request: %v", err)
        }

        part, err := writer.CreateFormFile("file", filepath.Base(filePath))
        if err != nil {
            return fmt.Errorf("error creating file field for Discord: %v", err)
        }

        if _, err := io.Copy(part, file); err != nil {
            return fmt.Errorf("error copying file to Discord request: %v", err)
        }

        if err := writer.Close(); err != nil {
            return fmt.Errorf("error finalizing Discord request: %v", err)
        }

        cm.log.Info("Sending clip to Discord. File: %s", filepath.Base(filePath))

        req, err := http.NewRequest("POST", webhookURL, &requestBody)
        if err != nil {
            return fmt.Errorf("error creating Discord request: %v", err)
        }

        req.Header.Set("Content-Type", writer.FormDataContentType())

        resp, err := cm.httpClient.Do(req)
        if err != nil {
            return fmt.Errorf("error sending to Discord: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 300 {
            bodyBytes, _ := io.ReadAll(resp.Body)
            return fmt.Errorf("discord API error: %s - %s", resp.Status, string(bodyBytes))
        }

        cm.log.Success("Clip successfully sent to Discord")
        return nil
    }

    return cm.RetryOperation(operation, "Discord")
}

func (cm *ClipManager) SendToChatApp(originalFilePath string, r *http.Request) error {
    chatApps := strings.ToLower(r.URL.Query().Get("chat_app"))
    if chatApps == "" && r.Method == http.MethodPost {
        var req ClipRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
            chatApps = strings.ToLower(req.ChatApps)
        }
        r.Body = io.NopCloser(bytes.NewBuffer([]byte{}))
    }

    chatAppList := strings.Split(chatApps, ",")

    var wg sync.WaitGroup
    errors := make(chan error, len(chatAppList))
    compressedFiles := make(map[string]string)

    for _, app := range chatAppList {
        app = strings.TrimSpace(app)

        filePath, err := cm.PrepareClipForChatApp(originalFilePath, app)
        if err != nil {
            cm.log.Error("Error preparing clip for %s: %v", app, err)
            errors <- fmt.Errorf("error preparing clip for %s: %v", app, err)
            continue
        }

        if filePath != originalFilePath {
            compressedFiles[app] = filePath
        }

        wg.Add(1)
        go func(app, filePath string) {
            defer wg.Done()

            var err error
            switch app {
            case "telegram":
                botToken := r.URL.Query().Get("telegram_bot_token")
                chatID := r.URL.Query().Get("telegram_chat_id")
                err = cm.sendToTelegram(filePath, botToken, chatID, r)
            case "mattermost":
                url := r.URL.Query().Get("mattermost_url")
                token := r.URL.Query().Get("mattermost_token")
                channel := r.URL.Query().Get("mattermost_channel")
                err = cm.sendToMattermost(filePath, url, token, channel, r)
            case "discord":
                webhookURL := r.URL.Query().Get("discord_webhook_url")
                err = cm.sendToDiscord(filePath, webhookURL, r)
            default:
                err = fmt.Errorf("unsupported chat app: %s", app)
            }

            if err != nil {
                cm.log.Error("Error sending clip to %s: %v", app, err)
                errors <- fmt.Errorf("error sending to %s: %v", app, err)
            } else {
                cm.log.Success("Successfully sent clip to %s", app)
            }
        }(app, filePath)
    }

    wg.Wait()
    close(errors)

    for app, filePath := range compressedFiles {
        cm.log.Info("Cleaning up compressed file for %s: %s", app, filePath)
        os.Remove(filePath)
    }

    var errList []string
    for err := range errors {
        errList = append(errList, err.Error())
    }

    if len(errList) > 0 {
        return fmt.Errorf("errors sending clip: %s", strings.Join(errList, "; "))
    }

    return nil
}

func (cm *ClipManager) buildClipMessage(r *http.Request) string {
    var category, team1, team2, additionalText string

    if r.Method == http.MethodGet {
        category = r.URL.Query().Get("category")
        team1 = r.URL.Query().Get("team1")
        team2 = r.URL.Query().Get("team2")
        additionalText = r.URL.Query().Get("additional_text")
    } else if r.Method == http.MethodPost {
        // Voor POST moeten we de body opnieuw parsen als we geen ClipRequest gebruiken
        var req ClipRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
            category = req.Category
            team1 = req.Team1
            team2 = req.Team2
            additionalText = req.AdditionalText
        }
        // Reset de body zodat deze opnieuw gelezen kan worden elders
        r.Body = io.NopCloser(bytes.NewBuffer([]byte{}))
    }

    base := fmt.Sprintf("New %sClip: %s", optionalCategory(category), cm.formatCurrentTime())

    var teams string
    if team1 != "" && team2 != "" {
        teams = fmt.Sprintf(" / %s vs %s", team1, team2)
    }

    var extra string
    if additionalText != "" {
        extra = fmt.Sprintf(" - %s", additionalText)
    }

    return base + teams + extra
}

// optionalCategory adds a space if category is present
func optionalCategory(category string) string {
	if category != "" {
		return category + " "
	}
	return ""
}

// formatCurrentTime returns a formatted current time string
func (cm *ClipManager) formatCurrentTime() string {
	return time.Now().Format("2006-01-02")
}

// serveWebInterface serves the HTML form interface at the root endpoint
func (cm *ClipManager) serveWebInterface(w http.ResponseWriter, r *http.Request) {
	templatePath := "templates/index.html"

	_, err := os.Stat(templatePath)
	if err != nil {
		execPath, err := os.Executable()
		if err == nil {
			execDir := filepath.Dir(execPath)
			templatePath = filepath.Join(execDir, "templates/index.html")
		}
	}

	htmlContent, err := os.ReadFile(templatePath)
	if err != nil {
		cm.log.Warning("Error reading template file: %v, using embedded HTML", err)
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
	log.Println("Starting ClipManager...")

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	cameraIP := os.Getenv("CAMERA_IP")
	if cameraIP == "" {
		log.Fatal("CAMERA_IP environment variable must be set")
	}

	containerPort := "5000"
	hostPort := getHostPort()
	if hostPort == "" {
		log.Fatal("HOST_PORT environment variable must be set")
	}

	clipManager, err := NewClipManager("clips", hostPort, cameraIP)
	if err != nil {
		log.Fatalf("Failed to initialize ClipManager: %v", err)
	}

	go clipManager.StartBackgroundRecording()

	os.MkdirAll("templates", 0755)
	os.MkdirAll("static/css", 0755)
	os.MkdirAll("static/img", 0755)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/api/clip", clipManager.RateLimit(clipManager.HandleClipRequest))
	http.HandleFunc("/", clipManager.serveWebInterface)

	clipManager.log.Info("ClipManager is running!")
	clipManager.log.Info("Access the web interface at: http://localhost:%s/", hostPort)
	clipManager.log.Info("API endpoint available at: http://localhost:%s/api/clip", hostPort)

	log.Fatal(http.ListenAndServe(":"+containerPort, nil))
}

func getHostPort() string {
	hostPort := os.Getenv("HOST_PORT")
	if hostPort == "" {
		return "5001"
	}
	return hostPort
}