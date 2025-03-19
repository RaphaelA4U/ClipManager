<?php
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\SettingsController;
use App\Http\Controllers\RecordController;

Route::post('/settings', [SettingsController::class, 'store']);
Route::get('/record', [RecordController::class, 'record']);