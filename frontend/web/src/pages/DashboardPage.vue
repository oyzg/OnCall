<template>
  <div class="dashboard-grid">
    <StatusCard
      title="Go API"
      :value="goHealth"
      description="核心业务后端基础链路状态"
    />
    <StatusCard
      title="Python AI"
      :value="pythonHealth"
      description="FastAPI / LangChain / LangGraph 基础服务状态"
    />
    <StatusCard
      title="Knowledge"
      value="pending"
      description="知识库与 RAG 主链路将在后续阶段接入"
    />
    <StatusCard
      title="Alerts"
      value="pending"
      description="告警中心和 AI 分析闭环将在业务阶段实现"
    />
  </div>
</template>

<script setup lang="ts">
import StatusCard from "@/components/StatusCard.vue";
import { useHealthStatus } from "@/composables/useHealthStatus";

const { goHealth, pythonHealth } = useHealthStatus();
</script>
