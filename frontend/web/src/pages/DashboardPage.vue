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
      :details="pythonHealthDetails"
    />
    <StatusCard
      title="Knowledge"
      value="integrated"
      description="知识库已接入 Embedding、Milvus、Elasticsearch 与本地兜底检索"
    />
    <StatusCard
      title="Alerts"
      value="ready"
      description="告警中心、AI 分析和关联排障会话已经形成闭环"
    />
  </div>
</template>

<script setup lang="ts">
import StatusCard from "@/components/StatusCard.vue";
import { useHealthStatus } from "@/composables/useHealthStatus";

const { goHealth, pythonHealth, pythonHealthDetails } = useHealthStatus();
</script>
