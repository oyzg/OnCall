<template>
  <div class="login-page">
    <section class="login-panel">
      <div class="login-copy">
        <span class="eyebrow">AI OnCall</span>
        <h1>智能值班平台</h1>
        <p>前端阶段 3 已进入真正的后台壳层建设，当前页面用于承接后续登录闭环。</p>
      </div>

      <el-card shadow="never" class="login-card">
        <el-form label-position="top" @submit.prevent>
          <el-form-item label="用户名">
            <el-input v-model="username" placeholder="请输入用户名" />
          </el-form-item>

          <el-form-item label="密码">
            <el-input v-model="password" type="password" show-password placeholder="请输入密码" />
          </el-form-item>

          <el-button type="primary" class="login-button" @click="handleLogin">
            进入系统
          </el-button>
        </el-form>
      </el-card>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";

import { useAuthStore } from "@/stores/auth";

const router = useRouter();
const authStore = useAuthStore();

const username = ref("");
const password = ref("");

function handleLogin() {
  authStore.setToken(`${username.value || "demo"}:${password.value || "demo"}`);
  router.push("/");
}
</script>
