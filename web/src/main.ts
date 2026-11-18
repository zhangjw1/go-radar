import { createApp } from 'vue';
import router from './router';
import store from './store';
import App from './App.vue';
import '@/assets/style/tailwind.css';
import '@/api/interceptor';

const app = createApp(App);

app.use(store);
app.use(router);

app.mount('#app');
