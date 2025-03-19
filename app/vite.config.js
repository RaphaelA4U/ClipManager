import { defineConfig } from 'vite';
import laravel from 'laravel-vite-plugin';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
    plugins: [
        laravel({
            input: 'resources/js/app.js',  // Verwijder array als het maar één bestand is
            refresh: true,
        }),
        vue(),
    ],
});