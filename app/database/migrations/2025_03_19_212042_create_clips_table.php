<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration {
    public function up()
    {
        Schema::create('clips', function (Blueprint $table) {
            $table->id();
            $table->string('user');
            $table->string('file_path');
            $table->string('chat_app');
            $table->timestamps();
        });
    }

    public function down()
    {
        Schema::dropIfExists('clips');
    }
};
