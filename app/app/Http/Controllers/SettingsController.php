<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use App\Models\Setting;
use Illuminate\Support\Facades\Log;

class SettingsController extends Controller
{
    public function store(Request $request)
    {
        $validated = $request->validate([
            'camera_ip' => 'required|string',
            'chat_app' => 'required|in:Mattermost,Discord,WhatsApp,Telegram',
            'bot_token' => 'required_if:chat_app,Telegram|string', // Alleen verplicht voor Telegram
            'chat_id' => 'required_if:chat_app,Telegram|string',   // Alleen verplicht voor Telegram
            'chat_token' => 'required_unless:chat_app,Telegram|string', // Voor andere chat-apps
            'log_retention' => 'required|integer|min:1',
            'video_retention' => 'required|integer|min:1',
        ]);

        // Als chat_app Telegram is, gebruiken we bot_token en chat_id
        if ($validated['chat_app'] === 'Telegram') {
            $validated['bot_token'] = $validated['bot_token'];
            $validated['chat_id'] = $validated['chat_id'];
            unset($validated['chat_token']); // Verwijder chat_token uit de data
        } else {
            // Voor andere chat-apps, gebruik chat_token als bot_token (chat_id blijft leeg)
            $validated['bot_token'] = $validated['chat_token'];
            $validated['chat_id'] = null;
            unset($validated['chat_token']);
        }

        // Update of maak instellingen
        $setting = Setting::first();
        if ($setting) {
            $setting->update($validated);
        } else {
            $setting = Setting::create($validated);
        }

        return response()->json(['message' => 'Instellingen opgeslagen']);
    }
}