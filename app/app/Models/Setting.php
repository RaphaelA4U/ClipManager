<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Setting extends Model
{
    protected $fillable = [
        'camera_ip',
        'chat_app',
        'bot_token', // Nieuwe veld
        'chat_id',   // Nieuwe veld
        'log_retention',
        'video_retention',
    ];
}