import { computed, ref } from "vue";
import { defineStore } from "pinia";

import { fetchCurrentUser, login, type AuthUser } from "@/services/api";
import { clearAccessToken, getAccessToken, setAccessToken } from "@/stores/session";

export const useAuthStore = defineStore("auth", () => {
  const token = ref<string>(getAccessToken());
  const user = ref<AuthUser | null>(null);

  const isAuthenticated = computed(() => Boolean(token.value));

  function setToken(value: string) {
    token.value = value;
    setAccessToken(value);
  }

  function clearToken() {
    token.value = "";
    user.value = null;
    clearAccessToken();
  }

  async function loginWithPassword(username: string, password: string) {
    const result = await login({ username, password });
    setToken(result.data.access_token.token);
    user.value = result.data.user;
    return result.data.user;
  }

  async function fetchMe() {
    if (!token.value) {
      return null;
    }

    try {
      const result = await fetchCurrentUser();
      user.value = result.data.user;
      return user.value;
    } catch (error) {
      clearToken();
      throw error;
    }
  }

  return {
    token,
    user,
    isAuthenticated,
    setToken,
    clearToken,
    loginWithPassword,
    fetchMe,
  };
});
