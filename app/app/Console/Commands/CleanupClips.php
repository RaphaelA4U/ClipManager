<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Setting;
use App\Models\Clip;

class CleanupClips extends Command
{
    protected $signature = 'clips:cleanup';
    protected $description = 'Verwijdert oude clips en logs';

    public function handle()
    {
        $setting = Setting::first();
        if (!$setting) {
            $this->error('Geen instellingen');
            return;
        }

        $clips = Clip::where('created_at', '<', now()->subDays($setting->log_retention))->get();
        foreach ($clips as $clip) {
            if (file_exists($clip->file_path) && now()->diffInDays($clip->created_at) > $setting->video_retention) {
                unlink($clip->file_path);
            }
            $clip->delete();
        }

        $this->info('Oude clips en logs verwijderd');
    }
}
