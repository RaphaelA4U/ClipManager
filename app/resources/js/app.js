import { createApp } from 'vue';
import App from './App.vue';
import { createVuetify } from 'vuetify';
import 'vuetify/styles';
import * as components from 'vuetify/components';
import * as directives from 'vuetify/directives';
import '@mdi/font/css/materialdesignicons.css';
import Toast from 'vue-toast-notification';
import 'vue-toast-notification/dist/theme-sugar.css';

const vuetify = createVuetify({ components, directives });
const app = createApp(App);
app.use(vuetify);
app.use(Toast);

// Add global $notify method
app.config.globalProperties.$notify = function(options) {
    this.$toast.open({
        message: options.text,
        type: options.type || 'info',
        position: 'top-right',
        duration: 3000,
    });
};

app.mount('#app');