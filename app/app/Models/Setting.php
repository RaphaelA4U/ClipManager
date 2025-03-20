<?php
namespace App\Models;
use Illuminate\Database\Eloquent\Model;

class Setting extends Model
{
    protected $fillable = [
        'id',
        'camera_ip',
        'chat_app',
        'chat_token',
        'log_retention',
        'video_retention',
    ];
}