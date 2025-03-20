package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type ClipRequest struct {
	CameraIP         string `json:"camera_ip"`
	ChatApp          string `json:"chat_app"`
	BotToken         string `json:"bot_token"`
	ChatID           string `json:"chat_id"`
	BacktrackSeconds int    `json:"backtrack_seconds"`
	DurationSeconds  int    `json:"duration_seconds"`
}

type ClipResponse struct {
	Message string `json:"message"`
}

func main() {
	// Laad .env-bestand
	err := godotenv.Load()
	if err != nil {
		log.Println("Geen .env-bestand gevonden, gebruik standaardwaarden")
	}

	// Haal de poort uit .env, standaard 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Stel de HTTP-server in
	http.HandleFunc("/api/clip", handleClipRequest)

	// Log het opstartbericht
	log.Printf("ClipManager gestart! Maak een GET/POST request naar localhost:%s/api/clip met parameters: camera_ip, chat_app, bot_token, chat_id, backtrack_seconds, duration_seconds", port)

	// Start de server
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleClipRequest(w http.ResponseWriter, r *http.Request) {
	// Accepteer zowel GET als POST
	var req ClipRequest

	if r.Method == http.MethodGet {
		// Parse query parameters voor GET
		req.CameraIP = r.URL.Query().Get("camera_ip")
		req.ChatApp = r.URL.Query().Get("chat_app")
		req.BotToken = r.URL.Query().Get("bot_token")
		req.ChatID = r.URL.Query().Get("chat_id")
		backtrackSeconds := r.URL.Query().Get("backtrack_seconds")
		durationSeconds := r.URL.Query().Get("duration_seconds")

		if backtrackSeconds != "" {
			fmt.Sscanf(backtrackSeconds, "%d", &req.BacktrackSeconds)
		}
		if durationSeconds != "" {
			fmt.Sscanf(durationSeconds, "%d", &req.DurationSeconds)
		}
	} else if r.Method == http.MethodPost {
		// Parse JSON-body voor POST
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Ongeldige JSON-body", http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "Methode niet toegestaan, gebruik GET of POST", http.StatusMethodNotAllowed)
		return
	}

	// Valideer de parameters
	if req.CameraIP == "" || req.ChatApp == "" || req.BotToken == "" || req.ChatID == "" {
		http.Error(w, "Missende parameters: camera_ip, chat_app, bot_token, en chat_id zijn verplicht", http.StatusBadRequest)
		return
	}
	if req.BacktrackSeconds < 5 || req.BacktrackSeconds > 300 {
		http.Error(w, "backtrack_seconds moet tussen 5 en 300 liggen", http.StatusBadRequest)
		return
	}
	if req.DurationSeconds < 5 || req.DurationSeconds > 300 {
		http.Error(w, "duration_seconds moet tussen 5 en 300 liggen", http.StatusBadRequest)
		return
	}
	if req.ChatApp != "Telegram" {
		http.Error(w, "Alleen Telegram wordt ondersteund als chat_app", http.StatusBadRequest)
		return
	}

	// Maak een tijdelijke directory voor de clip
	tempDir := "clips"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		http.Error(w, "Kon tijdelijke directory niet aanmaken", http.StatusInternalServerError)
		return
	}

	// Genereer een unieke bestandsnaam
	fileName := fmt.Sprintf("clip_%d.mp4", time.Now().Unix())
	filePath := filepath.Join(tempDir, fileName)
	compressedFilePath := filepath.Join(tempDir, "compressed_"+fileName)

	// Neem eerst de clip op zonder compressie
	outputArgs := ffmpeg.KwArgs{
		"ss":         req.BacktrackSeconds,
		"t":          req.DurationSeconds,
		"c:v":        "copy",  // Kopieer video zonder hercodering
		"c:a":        "copy",  // Kopieer audio zonder hercodering
		"movflags":   "+faststart",
	}

	// Neem de clip op met FFmpeg
	err := ffmpeg.Input(req.CameraIP, ffmpeg.KwArgs{"rtsp_transport": "tcp"}).
		Output(filePath, outputArgs).
		OverWriteOutput().
		Run()
	if err != nil {
		log.Printf("FFmpeg fout: %v", err)
		http.Error(w, "Kon de clip niet opnemen", http.StatusInternalServerError)
		return
	}

	// Controleer of het bestand bestaat en niet te klein is
	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() < 1024 {
		os.Remove(filePath) // Verwijder het bestand bij fout
		http.Error(w, "Kon de clip niet opnemen, bestand te klein", http.StatusInternalServerError)
		return
	}

	// Controleer bestandsgrootte en comprimeer alleen als > 50MB
	finalFilePath := filePath
	if fileInfo.Size() > 50*1024*1024 { // 50MB in bytes
		log.Printf("Bestand is groter dan 50MB (%d bytes), compressie wordt toegepast", fileInfo.Size())
		
		// Comprimeer naar 1920x1080
		err = ffmpeg.Input(filePath).
			Output(compressedFilePath, ffmpeg.KwArgs{
				"vf":      "scale=1920:1080",
				"c:v":     "libx264",
				"preset":  "medium",
				"crf":     "23",
				"c:a":     "aac",
				"b:a":     "128k",
				"movflags": "+faststart",
			}).
			OverWriteOutput().
			Run()
			
		if err != nil {
			log.Printf("Compressie fout: %v, origineel bestand wordt gebruikt", err)
		} else {
			// Gebruik het gecomprimeerde bestand en verwijder het origineel
			os.Remove(filePath)
			finalFilePath = compressedFilePath
		}
	}

	// Verstuur de clip naar Telegram (asynchroon)
	go func() {
		defer os.Remove(finalFilePath) // Zorg ervoor dat het bestand altijd wordt verwijderd

		file, err := os.Open(finalFilePath)
		if err != nil {
			log.Printf("Kon het bestand niet openen voor verzending: %v", err)
			return
		}
		defer file.Close()

		reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendVideo", req.BotToken)
		client := &http.Client{Timeout: 30 * time.Second}
		multipartReq, err := http.NewRequest("POST", reqURL, nil)
		if err != nil {
			log.Printf("Kon Telegram-verzoek niet aanmaken: %v", err)
			return
		}

		form := &multipartForm{
			fields: map[string]string{
				"chat_id": req.ChatID,
				"caption": "Nieuwe clip opgenomen!",
			},
			files: map[string]*os.File{
				"video": file,
			},
		}
		body, contentType, err := form.Build()
		if err != nil {
			log.Printf("Kon multipart-form niet aanmaken: %v", err)
			return
		}

		multipartReq.Header.Set("Content-Type", contentType)
		multipartReq.Body = body

		resp, err := client.Do(multipartReq)
		if err != nil {
			log.Printf("Fout bij verzenden naar Telegram: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Telegram API fout: %s", resp.Status)
			return
		}

		log.Printf("Clip succesvol verzonden naar Telegram")
	}()

	// Stuur direct een succesresponse
	response := ClipResponse{Message: "Clip opgenomen en verzending gestart"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper om multipart-form data te maken
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

	// Voeg velden toe
	for key, value := range mf.fields {
		if err := writer.WriteField(key, value); err != nil {
			bodyFile.Close()
			os.Remove(bodyFile.Name())
			return nil, "", err
		}
	}

	// Voeg bestanden toe
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

	// Sluit de writer om de boundary te schrijven
	writer.Close()

	// Zet de file pointer terug naar het begin
	if _, err := bodyFile.Seek(0, 0); err != nil {
		bodyFile.Close()
		os.Remove(bodyFile.Name())
		return nil, "", err
	}

	return bodyFile, writer.FormDataContentType(), nil
}
