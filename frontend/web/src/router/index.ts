import { createRouter, createWebHistory } from "vue-router";

import AppShell from "@/layouts/AppShell.vue";
import AlertsPage from "@/pages/AlertsPage.vue";
import AuditPage from "@/pages/AuditPage.vue";
import ChatPage from "@/pages/ChatPage.vue";
import DashboardPage from "@/pages/DashboardPage.vue";
import KnowledgePage from "@/pages/KnowledgePage.vue";
import LoginPage from "@/pages/LoginPage.vue";
import SettingsPage from "@/pages/SettingsPage.vue";
import { useAuthStore } from "@/stores/auth";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: LoginPage,
      meta: {
        public: true,
        title: "登录",
        description: "进入 AI OnCall 平台",
      },
    },
    {
      path: "/",
      component: AppShell,
      children: [
        {
          path: "",
          name: "dashboard",
          component: DashboardPage,
          meta: {
            title: "工作台",
            description: "查看系统健康、告警概览与 AI 服务状态",
          },
        },
        {
          path: "chat",
          name: "chat",
          component: ChatPage,
          meta: {
            title: "对话中心",
            description: "承载多轮问答、SSE 流式回复与工具增强会话",
          },
        },
        {
          path: "alerts",
          name: "alerts",
          component: AlertsPage,
          meta: {
            title: "告警中心",
            description: "集中查看告警、分析结果与处理时间线",
          },
        },
        {
          path: "knowledge",
          name: "knowledge",
          component: KnowledgePage,
          meta: {
            title: "知识库",
            description: "管理文档、检索测试与知识生命周期",
          },
        },
        {
          path: "audit",
          name: "audit",
          component: AuditPage,
          meta: {
            title: "审计日志",
            description: "查看用户操作、工具调用和模型行为轨迹",
          },
        },
        {
          path: "settings",
          name: "settings",
          component: SettingsPage,
          meta: {
            title: "系统配置",
            description: "维护模型、检索、工具与平台参数配置",
          },
        },
      ],
    },
  ],
});

router.beforeEach((to) => {
  const authStore = useAuthStore();

  if (to.meta.public) {
    return true;
  }

  if (!authStore.isAuthenticated) {
    return "/login";
  }

  return true;
});

export default router;
