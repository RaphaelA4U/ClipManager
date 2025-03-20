<template>
    <v-app :dark="darkMode">
        <v-main>
            <v-container class="pa-6">
                <v-row justify="center">
                    <v-col cols="12" md="8">
                        <h1 class="text-h4 font-weight-bold mb-2">ClipManager</h1>
                        <p class="text-subtitle-1 mb-6">
                            Configureer je camera en genereer QR-codes om clips op te nemen en te delen.
                        </p>

                        <!-- Instellingen Card -->
                        <v-card class="mb-6 elevation-3" rounded="lg">
                            <v-card-title class="text-h5 primary white--text">Instellingen</v-card-title>
                            <v-card-text class="pt-4">
                                <v-form @submit.prevent="saveSettings" ref="settingsForm">
                                    <v-text-field
                                        v-model="cameraIp"
                                        label="Camera IP"
                                        placeholder="bijv. rtsp://192.168.1.100:8554/live"
                                        prepend-icon="mdi-camera"
                                        :rules="[rules.required, rules.url]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-text-field>

                                    <v-select
                                        v-model="chatApp"
                                        :items="chatApps"
                                        label="Chat App"
                                        prepend-icon="mdi-chat"
                                        :rules="[rules.required]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-select>

                                    <v-text-field
                                        v-model="botToken"
                                        label="Bot Token"
                                        placeholder="bijv. 0123456789:ABCDEFGHIJKLMNOPQRSTUVWQYZABCDEFGHI"
                                        prepend-icon="mdi-robot"
                                        :rules="[rules.required]"
                                        outlined
                                        dense
                                        class="mb-4"
                                        v-if="chatApp === 'Telegram'"
                                    ></v-text-field>

                                    <v-text-field
                                        v-model="chatId"
                                        label="Chat ID"
                                        placeholder="bijv. -0123456789012"
                                        prepend-icon="mdi-account-group"
                                        :rules="[rules.required]"
                                        outlined
                                        dense
                                        class="mb-4"
                                        v-if="chatApp === 'Telegram'"
                                    ></v-text-field>

                                    <v-text-field
                                        v-model="chatToken"
                                        label="Chat Token/Webhook"
                                        placeholder="bijv. https://webhook-url"
                                        prepend-icon="mdi-webhook"
                                        :rules="[rules.required]"
                                        outlined
                                        dense
                                        class="mb-4"
                                        v-if="chatApp !== 'Telegram'"
                                    ></v-text-field>

                                    <v-text-field
                                        v-model="logRetention"
                                        label="Log Retentie (dagen)"
                                        type="number"
                                        prepend-icon="mdi-file-document"
                                        :rules="[rules.required, rules.min1]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-text-field>

                                    <v-text-field
                                        v-model="videoRetention"
                                        label="Video Retentie (dagen)"
                                        type="number"
                                        prepend-icon="mdi-video"
                                        :rules="[rules.required, rules.min1]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-text-field>

                                    <v-btn
                                        type="submit"
                                        color="primary"
                                        :loading="loading"
                                        :disabled="loading"
                                        rounded
                                        elevation="2"
                                    >
                                        Opslaan
                                    </v-btn>
                                </v-form>
                            </v-card-text>
                        </v-card>

                        <!-- Record Clip Card -->
                        <v-card class="mb-6 elevation-3" rounded="lg">
                            <v-card-title class="text-h5 primary white--text">Clip Opnemen</v-card-title>
                            <v-card-text class="pt-4">
                                <v-form @submit.prevent="recordClip" ref="recordForm">
                                    <v-text-field
                                        v-model="qrUser"
                                        label="Gebruiker"
                                        placeholder="bijv. Henk"
                                        prepend-icon="mdi-account"
                                        :rules="[rules.required]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-text-field>

                                    <v-text-field
                                        v-model="qrBacktrack"
                                        label="Seconden Terug"
                                        type="number"
                                        prepend-icon="mdi-rewind"
                                        :rules="[rules.required, rules.range5to60]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-text-field>

                                    <v-text-field
                                        v-model="qrDuration"
                                        label="Duur (seconden)"
                                        type="number"
                                        prepend-icon="mdi-timer"
                                        :rules="[rules.required, rules.range5to60]"
                                        outlined
                                        dense
                                        class="mb-4"
                                    ></v-text-field>

                                    <v-btn
                                        type="submit"
                                        color="primary"
                                        :loading="loading"
                                        :disabled="loading"
                                        rounded
                                        elevation="2"
                                    >
                                        Clip Opnemen
                                    </v-btn>
                                </v-form>

                                <!-- QR Code Result -->
                                <v-card v-if="qrUrl" class="mt-5 elevation-2" rounded="lg">
                                    <v-card-text class="text-center">
                                        <p class="mb-2">
                                            <a :href="qrUrl" target="_blank" class="text-decoration-none primary--text">{{ qrUrl }}</a>
                                        </p>
                                        <v-img v-if="qrImage" :src="qrImage" alt="QR Code" max-width="200" class="mx-auto"/>
                                    </v-card-text>
                                </v-card>
                            </v-card-text>
                        </v-card>

                        <!-- Dark Mode Toggle -->
                        <v-btn
                            fab
                            small
                            color="primary"
                            class="dark-mode-toggle"
                            @click="darkMode = !darkMode"
                        >
                            <v-icon>{{ darkMode ? 'mdi-white-balance-sunny' : 'mdi-moon-waxing-crescent' }}</v-icon>
                        </v-btn>
                    </v-col>
                </v-row>
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
            darkMode: true, // Start in dark mode
            cameraIp: '',
            chatApp: '',
            chatApps: ['Mattermost', 'Discord', 'WhatsApp', 'Telegram'],
            botToken: '',
            chatId: '',
            chatToken: '',
            logRetention: 7,
            videoRetention: 1,
            qrUser: '',
            qrBacktrack: 20,
            qrDuration: 10,
            qrUrl: '',
            qrImage: '',
            loading: false, // Voor laadan feedback
            rules: {
                required: (value) => !!value || 'Dit veld is verplicht',
                url: (value) => {
                    const pattern = /^(rtsp|http|https):\/\/[^\s$.?#].[^\s]*$/;
                    return pattern.test(value) || 'Voer een geldige URL in (bijv. rtsp://192.168.1.100:8554/live)';
                },
                min1: (value) => (Number(value) >= 1) || 'Waarde moet minimaal 1 zijn',
                range5to60: (value) => (Number(value) >= 5 && Number(value) <= 60) || 'Waarde moet tussen 5 en 60 liggen',
            },
        };
    },
    methods: {
        async saveSettings() {
            if (!this.$refs.settingsForm.validate()) {
                return;
            }

            this.loading = true;
            try {
                const data = {
                    camera_ip: this.cameraIp,
                    chat_app: this.chatApp,
                    log_retention: Number(this.logRetention),
                    video_retention: Number(this.videoRetention),
                };

                // Voeg bot_token en chat_id toe voor Telegram, anders chat_token
                if (this.chatApp === 'Telegram') {
                    data.bot_token = this.botToken;
                    data.chat_id = this.chatId;
                } else {
                    data.chat_token = this.chatToken;
                }

                const response = await axios.post('/api/settings', data);
                this.$notify({
                    type: 'success',
                    text: 'Instellingen succesvol opgeslagen!',
                });
            } catch (error) {
                console.error(error);
                let errorMessage = 'Fout bij opslaan: ' + error.message;
                if (error.response) {
                    if (error.response.status === 422) {
                        errorMessage = 'Validatiefout: ' + JSON.stringify(error.response.data.errors);
                    } else if (error.response.status === 404) {
                        errorMessage = 'API-endpoint niet gevonden. Controleer de serverconfiguratie.';
                    }
                }
                this.$notify({
                    type: 'error',
                    text: errorMessage,
                });
            } finally {
                this.loading = false;
            }
        },
        async recordClip() {
            if (!this.$refs.recordForm.validate()) {
                return;
            }

            this.loading = true;
            const url = `/api/record?user=${this.qrUser}&backtrack=${this.qrBacktrack}&duration=${this.qrDuration}&chat_app=${this.chatApp}`;
            this.qrUrl = url;

            try {
                const response = await axios.get(url);
                this.$notify({
                    type: 'success',
                    text: 'Opname gestart! Status: ' + response.data.message,
                });

                // Genereer QR-code
                this.qrImage = await QRCode.toDataURL(url);
            } catch (error) {
                console.error(error);
                const errorMessage = error.response?.data?.message || error.message;
                this.$notify({
                    type: 'error',
                    text: 'Fout bij starten opname: ' + errorMessage,
                });
            } finally {
                this.loading = false;
            }
        },
    },
};
</script>

<style scoped>
.dark-mode-toggle {
    position: fixed;
    bottom: 20px;
    right: 20px;
}
</style>