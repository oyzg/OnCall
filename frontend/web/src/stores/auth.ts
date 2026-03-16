import { computed, ref } from "vue";
import { defineStore } from "pinia";

const TOKEN_KEY = "oncall_token";

export const useAuthStore = defineStore("auth", () => {
  const token = ref<string>(localStorage.getItem(TOKEN_KEY) || "");

  const isAuthenticated = computed(() => Boolean(token.value));

  function setToken(value: string) {
    token.value = value;
    localStorage.setItem(TOKEN_KEY, value);
  }

  function clearToken() {
    token.value = "";
    localStorage.removeItem(TOKEN_KEY);
  }

  return {
    token,
    isAuthenticated,
    setToken,
    clearToken,
  };
});
