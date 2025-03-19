<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use App\Models\Setting;

class SettingsController extends Controller
{
    public function store(Request $request)
    {
        $validated = $request->validate([
            'camera_ip' => 'required|url',
            'chat_app' => 'required|in:Mattermost,Discord,WhatsApp,Telegram',
            'chat_token' => 'required|string',
            'log_retention' => 'required|integer|min:1',
            'video_retention' => 'required|integer|min:1',
        ]);

        $setting = Setting::updateOrCreate(['id' => 1], $validated);
        return response()->json($setting);
    }
}
