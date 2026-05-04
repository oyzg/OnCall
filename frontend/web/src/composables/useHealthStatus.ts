import { onMounted, ref } from "vue";

import { fetchGoHealth, fetchPythonHealth } from "@/services/api";

export function useHealthStatus() {
  const goHealth = ref("loading");
  const pythonHealth = ref("loading");
  const pythonHealthDetails = ref<string[]>([]);

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
      pythonHealthDetails.value = (python?.data?.components || [])
        .map((component: { name: string; status: string; detail?: string }) =>
          `${component.name}: ${component.status}${component.detail ? ` (${component.detail})` : ""}`
        )
        .slice(0, 4);
    } catch {
      pythonHealth.value = "unavailable";
      pythonHealthDetails.value = [];
    }
  }

  onMounted(load);

  return {
    goHealth,
    pythonHealth,
    pythonHealthDetails,
    reloadHealth: load,
  };
}
