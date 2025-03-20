<?php
namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Clip extends Model
{
    protected $fillable = [
        'user',
        'file_path',
        'chat_app',
    ];
}