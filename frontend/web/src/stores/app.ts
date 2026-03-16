import { computed, ref } from "vue";
import { defineStore } from "pinia";

import type { NavigationItem } from "@/types/navigation";

const navigation: NavigationItem[] = [
  { key: "dashboard", label: "工作台", path: "/" },
  { key: "chat", label: "对话中心", path: "/chat" },
  { key: "alerts", label: "告警中心", path: "/alerts" },
  { key: "knowledge", label: "知识库", path: "/knowledge" },
  { key: "audit", label: "审计日志", path: "/audit" },
  { key: "settings", label: "系统配置", path: "/settings" },
];

export const useAppStore = defineStore("app", () => {
  const title = ref("AI OnCall");
  const collapsed = ref(false);

  const menus = computed(() => navigation);

  function toggleSidebar() {
    collapsed.value = !collapsed.value;
  }

  return {
    title,
    collapsed,
    menus,
    toggleSidebar,
  };
});
