import { onMounted, ref } from "vue";

import { fetchGoHealth, fetchPythonHealth } from "@/services/api";

export function useHealthStatus() {
  const goHealth = ref("loading");
  const pythonHealth = ref("loading");

  async function load() {
    try {
      const go = await fetchGoHealth();
      goHealth.value = go?.data?.status || go?.status || "ok";
    } catch {
      goHealth.value = "unavailable";
    }

    try {
      const python = await fetchPythonHealth();
      pythonHealth.value = python?.data?.status || python?.status || "ok";
    } catch {
      pythonHealth.value = "unavailable";
    }
  }

  onMounted(load);

  return {
    goHealth,
    pythonHealth,
    reloadHealth: load,
  };
}
