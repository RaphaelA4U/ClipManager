<?php
namespace App\Http\Controllers;

use Illuminate\Http\Request;
use App\Models\Setting;
use App\Models\Clip;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Log;

class RecordController extends Controller
{
    public function record(Request $request)
    {
        $validated = $request->validate([
            'user' => 'required|string',
            'backtrack' => 'required|integer|min:5|max:60',
            'duration' => 'required|integer|min:5|max:60',
            'chat_app' => 'required|in:Mattermost,Discord,WhatsApp,Telegram',
        ]);

        $setting = Setting::first();
        if (!$setting) {
            return response()->json(['error' => 'Instellingen niet geconfigureerd'], 400);
        }

        $clipsDir = storage_path('app/public/clips');
        if (!file_exists($clipsDir)) {
            mkdir($clipsDir, 0755, true);
        }

        $fileName = "clip_" . time() . "_" . $validated['user'] . ".mp4";
        $filePath = "clips/{$fileName}";
        $fullPath = storage_path("app/public/{$filePath}");

        $command = "ffmpeg -rtsp_transport tcp -i {$setting->camera_ip} -ss {$validated['backtrack']} -t {$validated['duration']} -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart {$fullPath} 2>&1";
        
        exec($command, $output, $returnCode);
        Log::info("FFmpeg output: " . implode("\n", $output));
        
        if ($returnCode !== 0 || !file_exists($fullPath) || filesize($fullPath) < 1024) {
            Log::error("FFmpeg failed: " . implode("\n", $output));
            return response()->json(['error' => 'Kon de clip niet opnemen. Controleer de camera-IP en FFmpeg-logboeken.'], 500);
        }

        $clip = Clip::create([
            'user' => $validated['user'],
            'file_path' => $filePath,
            'chat_app' => $validated['chat_app'],
        ]);

        try {
            $this->sendToChatApp($fullPath, $validated['chat_app'], $setting->bot_token, $setting->chat_id);
        } catch (\Exception $e) {
            Log::error("Chatapp verzending mislukt: " . $e->getMessage());
            return response()->json(['message' => 'Clip opgeslagen, maar verzending naar chatapp mislukt.', 'clip_id' => $clip->id], 200);
        }

        return response()->json(['message' => 'Clip opgeslagen en verzonden', 'clip_id' => $clip->id]);
    }

    private function compressClip($filePath)
    {
        $compressedPath = str_replace('.mp4', '_compressed.mp4', $filePath);
        $command = "ffmpeg -i {$filePath} -vf scale=1280:720 -b:v 1M -r 30 -c:a aac -b:a 128k {$compressedPath} 2>&1";
        exec($command, $output, $returnCode);

        if ($returnCode !== 0) {
            Log::error("Compressie mislukt: " . implode("\n", $output));
            return $filePath; // Gebruik origineel als compressie faalt
        }

        return $compressedPath;
    }

    private function sendToChatApp($filePath, $chatApp, $botToken, $chatId)
    {
        $fileName = basename($filePath);
        $fileSize = filesize($filePath) / (1024 * 1024); // Grootte in MB

        Log::info("Bestandsgrootte van {$fileName}: {$fileSize} MB");

        if ($chatApp === 'Telegram') {
            if (!$botToken || !$chatId) {
                Log::error("Bot token of chat ID ontbreekt in instellingen");
                throw new \Exception("Bot token of chat ID ontbreekt in instellingen");
            }

            if ($fileSize > 50) {
                $filePath = $this->compressClip($filePath);
                $fileName = basename($filePath);
            }

            if (!is_readable($filePath)) {
                Log::error("Bestand {$filePath} is niet leesbaar");
                throw new \Exception("Bestand {$filePath} is niet leesbaar");
            }

            Log::info("Verstuur clip naar Telegram", [
                'botToken' => $botToken,
                'chatId' => $chatId,
                'filePath' => $filePath,
                'fileName' => $fileName,
                'url' => "https://api.telegram.org/bot{$botToken}/sendVideo"
            ]);

            $multipart = [
                [
                    'name' => 'chat_id',
                    'contents' => $chatId
                ],
                [
                    'name' => 'caption',
                    'contents' => 'Nieuwe clip opgenomen!'
                ],
                [
                    'name' => 'video',
                    'contents' => fopen($filePath, 'r'),
                    'filename' => $fileName
                ]
            ];

            $response = Http::timeout(30)
                ->asMultipart()
                ->post("https://api.telegram.org/bot{$botToken}/sendVideo", $multipart);

            if (!$response->successful()) {
                Log::error("Verzending naar Telegram mislukt: " . $response->body());
                throw new \Exception("Verzending naar Telegram mislukt: " . $response->body());
            }

            Log::info("Clip succesvol verzonden naar Telegram: " . $response->body());
            return;
        }

        throw new \Exception("Niet-ondersteunde chatapp: {$chatApp}");
    }
}