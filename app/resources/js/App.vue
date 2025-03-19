<template>
    <v-app>
        <v-main>
            <v-container>
                <h1>ClipManager</h1>
                <p>Configureer je camera en genereer QR-codes om clips op te nemen en te delen.</p>

                <v-card class="mt-5">
                    <v-card-title>Instellingen</v-card-title>
                    <v-card-text>
                        <v-form @submit.prevent="saveSettings">
                            <v-text-field v-model="cameraIp" label="Camera IP (bijv. http://192.168.1.100:8080)" required></v-text-field>
                            <v-select v-model="chatApp" :items="chatApps" label="Chat App" required></v-select>
                            <v-text-field v-model="chatToken" label="Chat Token/Webhook" required></v-text-field>
                            <v-text-field v-model="logRetention" label="Log Retentie (dagen)" type="number" min="1" required></v-text-field>
                            <v-text-field v-model="videoRetention" label="Video Retentie (dagen)" type="number" min="1" required></v-text-field>
                            <v-btn type="submit" color="primary">Opslaan</v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>

                <v-card class="mt-5">
                    <v-card-title>QR Generator</v-card-title>
                    <v-card-text>
                        <v-form @submit.prevent="generateQr">
                            <v-text-field v-model="qrUser" label="Gebruiker (bijv. Henk)" required></v-text-field>
                            <v-text-field v-model="qrBacktrack" label="Seconden Terug" type="number" min="5" max="60" required></v-text-field>
                            <v-text-field v-model="qrDuration" label="Duur (seconden)" type="number" min="5" max="60" required></v-text-field>
                            <v-btn type="submit" color="primary">Genereer</v-btn>
                        </v-form>
                        <v-card v-if="qrUrl" class="mt-5">
                            <v-card-text>
                                <p><a :href="qrUrl" target="_blank">{{ qrUrl }}</a></p>
                                <img v-if="qrImage" :src="qrImage" alt="QR Code" />
                            </v-card-text>
                        </v-card>
                    </v-card-text>
                </v-card>
            </v-container>
        </v-main>
    </v-app>
</template>

<script>
import axios from 'axios';
import QRCode from 'qrcode';

export default {
    name: 'ClipManager',
    data() {
        return {
            cameraIp: '',
            chatApp: '',
            chatApps: ['Mattermost', 'Discord', 'WhatsApp', 'Telegram'],
            chatToken: '',
            logRetention: 7,
            videoRetention: 1,
            qrUser: '',
            qrBacktrack: 20,
            qrDuration: 10,
            qrUrl: '',
            qrImage: '',
        };
    },
    methods: {
        async saveSettings() {
            // Client-side validatie
            if (!this.cameraIp || !this.cameraIp.match(/^https?:\/\/.+/)) {
                alert('Vul een geldige Camera IP in (bijv. http://example.com)');
                return;
            }
            if (!this.chatApp) {
                alert('Kies een Chat app');
                return;
            }
            if (!this.chatToken) {
                alert('Vul een Chat token in');
                return;
            }
            if (!Number.isInteger(Number(this.logRetention)) || this.logRetention < 1) {
                alert('Log retentie moet een getal zijn >= 1');
                return;
            }
            if (!Number.isInteger(Number(this.videoRetention)) || this.videoRetention < 1) {
                alert('Video retentie moet een getal zijn >= 1');
                return;
            }

            try {
                const response = await axios.post('/api/settings', {
                    camera_ip: this.cameraIp,
                    chat_app: this.chatApp,
                    chat_token: this.chatToken,
                    log_retention: Number(this.logRetention), // Forceer integer
                    video_retention: Number(this.videoRetention), // Forceer integer
                });
                alert('Instellingen opgeslagen!');
            } catch (error) {
                console.error(error);
                if (error.response && error.response.status === 422) {
                    alert('Validatiefout: ' + JSON.stringify(error.response.data.errors));
                } else if (error.response && error.response.status === 404) {
                    alert('API-endpoint niet gevonden. Controleer de serverconfiguratie.');
                } else {
                    alert('Fout bij opslaan: ' + error.message);
                }
            }
        },
        async generateQr() {
            this.qrUrl = `${window.location.origin}/api/record?user=${this.qrUser}&backtrack=${this.qrBacktrack}&duration=${this.qrDuration}&chat_app=${this.chatApp}`;
            try {
                this.qrImage = await QRCode.toDataURL(this.qrUrl);
            } catch (error) {
                console.error(error);
                alert('Fout bij genereren QR');
            }
        },
    },
};
</script>
