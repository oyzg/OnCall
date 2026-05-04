import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import { installElementPlus } from "./plugins/element-plus";
import router from "./router";
import "./styles/index.css";

const app = createApp(App);

app.use(createPinia());
app.use(router);
installElementPlus(app);
app.mount("#app");
