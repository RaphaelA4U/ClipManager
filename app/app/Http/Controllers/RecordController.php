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

        $setting = Setting::first();
        if (!$setting) {
            return response()->json(['error' => 'Instellingen niet geconfigureerd'], 400);
        }

        // Voorlopig een dummy clip, later FFmpeg toevoegen
        $filePath = storage_path('app/clips/clip_' . time() . '_' . $validated['user'] . '.mp4');
        file_put_contents($filePath, 'DUMMY_VIDEO_DATA');

        // Log de clip
        $clip = Clip::create([
            'user' => $validated['user'],
            'file_path' => $filePath,
            'chat_app' => $validated['chat_app'],
        ]);

        // Verstuur naar chat-app
        $this->sendToChatApp($filePath, $validated['chat_app'], $setting->chat_token);

        return response()->json(['message' => 'Clip opgeslagen en verzonden', 'clip_id' => $clip->id]);
    }

    private function sendToChatApp($filePath, $chatApp, $token)
    {
        switch ($chatApp) {
            case 'Mattermost':
            case 'Discord':
                Http::attach('file', file_get_contents($filePath), 'clip.mp4')
                    ->post($token);
                break;
            case 'WhatsApp':
                // Vereist Twilio API, voor nu dummy
                break;
            case 'Telegram':
                Http::attach('video', file_get_contents($filePath), 'clip.mp4')
                    ->post("https://api.telegram.org/bot{$token}/sendVideo", [
                        'chat_id' => 'YOUR_CHAT_ID', // Moet nog dynamisch worden
                    ]);
                break;
        }
    }
}