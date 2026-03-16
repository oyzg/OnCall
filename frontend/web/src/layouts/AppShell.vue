<template>
  <div class="app-shell">
    <aside :class="['app-sidebar', { collapsed: appStore.collapsed }]">
      <div class="brand">
        <span class="brand-mark">AI</span>
        <div v-if="!appStore.collapsed" class="brand-copy">
          <strong>OnCall</strong>
          <small>Smart Ops Console</small>
        </div>
      </div>

      <nav class="nav-list">
        <RouterLink
          v-for="item in appStore.menus"
          :key="item.key"
          :to="item.path"
          class="nav-item"
          active-class="active"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>

    <main class="app-main">
      <header class="app-header">
        <div>
          <h1>{{ pageTitle }}</h1>
          <p>{{ pageDescription }}</p>
        </div>

        <div class="header-actions">
          <el-button text @click="appStore.toggleSidebar()">
            {{ appStore.collapsed ? "展开导航" : "收起导航" }}
          </el-button>
          <el-button type="primary" plain @click="handleLogout">退出</el-button>
        </div>
      </header>

      <section class="app-content">
        <router-view />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";

import { useAppStore } from "@/stores/app";
import { useAuthStore } from "@/stores/auth";

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const authStore = useAuthStore();

const pageTitle = computed(() => (route.meta.title as string) || "AI OnCall");
const pageDescription = computed(
  () => (route.meta.description as string) || "Intelligent on-call platform"
);

function handleLogout() {
  authStore.clearToken();
  router.push("/login");
}
</script>
