import { createRouter, createWebHistory } from "vue-router";
import DashboardPage from "@/pages/DashboardPage.vue";
import LoginPage from "@/pages/LoginPage.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: LoginPage },
    { path: "/", name: "dashboard", component: DashboardPage },
  ],
});

export default router;

