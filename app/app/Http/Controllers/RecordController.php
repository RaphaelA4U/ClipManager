<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use App\Models\Setting;
use App\Models\Clip;
use Illuminate\Support\Facades\Http;

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

        // Haal instellingen op
        $setting = Setting::first();
        if (!$setting) {
            return response()->json(['error' => 'Instellingen niet geconfigureerd'], 400);
        }

        // Maak de clips-directory als die niet bestaat
        $clipsDir = storage_path('app/public/clips');
        if (!file_exists($clipsDir)) {
            mkdir($clipsDir, 0755, true);
        }

        // Genereer bestandsnaam en pad
        $filePath = "clips/clip_" . time() . "_" . $validated['user'] . ".mp4";
        $fullPath = storage_path("app/public/{$filePath}");

        // FFmpeg commando
        $command = "ffmpeg -i {$setting->camera_ip} -ss {$validated['backtrack']} -t {$validated['duration']} -c:v copy -c:a copy {$fullPath} 2>&1";
        
        exec($command, $output, $returnCode);
        
        // Controleer of FFmpeg succesvol was
        if ($returnCode !== 0) {
            return response()->json(['error' => 'Kon de clip niet opnemen. Controleer de camera-IP en FFmpeg-logboeken.'], 500);
        }

        // Sla de clip op in de database
        $clip = Clip::create([
            'user' => $validated['user'],
            'file_path' => $filePath,
            'chat_app' => $validated['chat_app'],
        ]);

        // Verstuur naar chatapp
        try {
            $this->sendToChatApp($fullPath, $validated['chat_app'], $setting->chat_token);
        } catch (\Exception $e) {
            return response()->json(['message' => 'Clip opgeslagen, maar verzending naar chatapp mislukt.', 'clip_id' => $clip->id], 200);
        }

        return response()->json(['message' => 'Clip opgeslagen en verzonden', 'clip_id' => $clip->id]);
    }

    private function sendToChatApp($filePath, $chatApp, $chatToken)
    {
        $fileName = basename($filePath);

        switch ($chatApp) {
            case 'Mattermost':
                // Mattermost webhook
                $response = Http::attach(
                    'attachment', file_get_contents($filePath), $fileName
                )->post($chatToken, [
                    'message' => 'Nieuwe clip opgenomen!',
                ]);
                break;

            case 'Discord':
                // Discord webhook
                $response = Http::attach(
                    'file', file_get_contents($filePath), $fileName
                )->post($chatToken, [
                    'content' => 'Nieuwe clip opgenomen!',
                ]);
                break;

            case 'Telegram':
                // Telegram bot API (chatToken moet in formaat zijn: "bot<token>:<chat_id>")
                [$botToken, $chatId] = explode(':', $chatToken);
                $response = Http::attach(
                    'video', file_get_contents($filePath), $fileName
                // )->post("https://api.telegram.org/{$botToken}/sendVideo", [
                //     'chat_id' => $chatId,
                )->post("https://api.telegram.org/7858892775:AAGGlqN5HwB5zzf0LVHNxS7aLCnlGELnI1w/sendVideo", [ //DEV
                    'chat_id' => '5597488890',                                                                 //DEV                                              
                    'caption' => 'Nieuwe clip opgenomen!',
                ]);
                break;

            case 'WhatsApp':
                // WhatsApp (vereist een service zoals Twilio of WhatsApp Business API)
                // Dit is een vereenvoudigd voorbeeld; je hebt een WhatsApp Business API setup nodig
                $response = Http::post('https://api.whatsapp.com/v1/messages', [
                    'to' => 'telefoonnummer', // Vervang door het nummer
                    'type' => 'video',
                    'video' => [
                        'link' => asset("storage/{$filePath}"), // Zorg dat de clip publiek toegankelijk is
                    ],
                ], [
                    'Authorization' => "Bearer {$chatToken}",
                ]);
                break;

            default:
                throw new \Exception("Niet-ondersteunde chatapp: {$chatApp}");
        }

        if (!$response->successful()) {
            throw new \Exception("Verzending naar {$chatApp} mislukt: " . $response->body());
        }
    }
}