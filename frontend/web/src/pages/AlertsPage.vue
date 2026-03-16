<template>
  <div class="alerts-page">
    <section class="alerts-toolbar">
      <div class="section-heading">
        <div>
          <h2>告警工作台</h2>
          <p>查看告警、处理状态、操作记录，并从告警直接进入排障会话。</p>
        </div>
        <el-button plain @click="loadAlerts">刷新</el-button>
      </div>

      <div class="filter-row">
        <el-select v-model="filters.status" clearable placeholder="全部状态" @change="loadAlerts">
          <el-option label="open" value="open" />
          <el-option label="acknowledged" value="acknowledged" />
          <el-option label="investigating" value="investigating" />
          <el-option label="resolved" value="resolved" />
        </el-select>
        <el-select v-model="filters.severity" clearable placeholder="全部级别" @change="loadAlerts">
          <el-option label="P0" value="P0" />
          <el-option label="P1" value="P1" />
          <el-option label="P2" value="P2" />
          <el-option label="P3" value="P3" />
        </el-select>
        <el-input v-model="filters.service" placeholder="按服务筛选" @keyup.enter="loadAlerts" />
      </div>
    </section>

    <section class="alerts-layout">
      <div class="alerts-table-card">
        <el-table :data="alerts" stripe @row-click="handleSelectAlert">
          <el-table-column prop="title" label="告警标题" min-width="260" />
          <el-table-column prop="service" label="服务" width="140" />
          <el-table-column prop="environment" label="环境" width="120" />
          <el-table-column label="级别" width="100">
            <template #default="{ row }">
              <el-tag :type="severityTagType(row.severity)">{{ row.severity }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="140">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="source" label="来源" width="130" />
          <el-table-column label="触发时间" min-width="180">
            <template #default="{ row }">
              {{ formatTime(row.triggered_at) }}
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="alerts-detail-card">
        <template v-if="activeAlert">
          <div class="detail-header">
            <div>
              <h3>{{ activeAlert.title }}</h3>
              <p>{{ activeAlert.summary }}</p>
            </div>
            <div class="detail-actions">
              <el-button
                type="primary"
                plain
                :disabled="linkingSession"
                @click="handleLinkSession"
              >
                {{ activeAlert.linked_session_id ? "进入排障会话" : "创建排障会话" }}
              </el-button>
            </div>
          </div>

          <el-descriptions :column="2" border class="detail-descriptions">
            <el-descriptions-item label="服务">{{ activeAlert.service }}</el-descriptions-item>
            <el-descriptions-item label="环境">{{ activeAlert.environment }}</el-descriptions-item>
            <el-descriptions-item label="级别">{{ activeAlert.severity }}</el-descriptions-item>
            <el-descriptions-item label="状态">{{ activeAlert.status }}</el-descriptions-item>
            <el-descriptions-item label="来源">{{ activeAlert.source }}</el-descriptions-item>
            <el-descriptions-item label="触发时间">{{ formatTime(activeAlert.triggered_at) }}</el-descriptions-item>
            <el-descriptions-item label="关联会话" :span="2">
              {{ activeAlert.linked_session_id || "尚未关联" }}
            </el-descriptions-item>
            <el-descriptions-item label="描述" :span="2">
              {{ activeAlert.description || "暂无详细描述" }}
            </el-descriptions-item>
          </el-descriptions>

          <div class="status-panel">
            <el-select v-model="nextStatus" placeholder="更新状态">
              <el-option label="open" value="open" />
              <el-option label="acknowledged" value="acknowledged" />
              <el-option label="investigating" value="investigating" />
              <el-option label="resolved" value="resolved" />
            </el-select>
            <el-input
              v-model="statusComment"
              placeholder="补充处理说明，可选"
              @keyup.enter="handleUpdateStatus"
            />
            <el-button type="primary" :loading="updatingStatus" @click="handleUpdateStatus">
              更新状态
            </el-button>
          </div>

          <div class="timeline-block">
            <h4>处理时间线</h4>
            <el-timeline>
              <el-timeline-item
                v-for="record in alertRecords"
                :key="record.id"
                :timestamp="formatTime(record.created_at)"
                placement="top"
              >
                <strong>{{ record.action }}</strong>
                <p>{{ record.comment }}</p>
                <small>{{ record.operator }}</small>
              </el-timeline-item>
            </el-timeline>
          </div>
        </template>

        <el-empty v-else description="选择左侧告警后，这里会展示详情和处理记录。" />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useRouter } from "vue-router";

import {
  fetchAlertDetail,
  fetchAlerts,
  linkAlertSession,
  updateAlertStatus,
  type AlertItem,
  type AlertRecord,
} from "@/services/api";

const router = useRouter();
const alerts = ref<AlertItem[]>([]);
const activeAlert = ref<AlertItem | null>(null);
const alertRecords = ref<AlertRecord[]>([]);
const updatingStatus = ref(false);
const linkingSession = ref(false);
const nextStatus = ref("");
const statusComment = ref("");

const filters = reactive({
  status: "",
  severity: "",
  service: "",
});

onMounted(async () => {
  await loadAlerts();
});

async function loadAlerts() {
  const result = await fetchAlerts({
    status: filters.status || undefined,
    severity: filters.severity || undefined,
    service: filters.service.trim() || undefined,
  });
  alerts.value = result.data.alerts;

  if (!activeAlert.value && alerts.value.length > 0) {
    await handleSelectAlert(alerts.value[0]);
    return;
  }

  if (activeAlert.value) {
    const exists = alerts.value.find((item) => item.id === activeAlert.value?.id);
    if (exists) {
      await handleSelectAlert(exists);
    } else {
      activeAlert.value = null;
      alertRecords.value = [];
    }
  }
}

async function handleSelectAlert(alert: AlertItem) {
  const result = await fetchAlertDetail(alert.id);
  activeAlert.value = result.data.alert;
  alertRecords.value = result.data.records;
  nextStatus.value = result.data.alert.status;
}

async function handleUpdateStatus() {
  if (!activeAlert.value || !nextStatus.value) {
    return;
  }

  updatingStatus.value = true;
  try {
    const result = await updateAlertStatus(activeAlert.value.id, {
      status: nextStatus.value,
      comment: statusComment.value,
    });
    activeAlert.value = result.data.alert;
    alertRecords.value = result.data.records;
    statusComment.value = "";
    ElMessage.success("告警状态已更新");
    await loadAlerts();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "状态更新失败");
  } finally {
    updatingStatus.value = false;
  }
}

async function handleLinkSession() {
  if (!activeAlert.value) {
    return;
  }

  linkingSession.value = true;
  try {
    const result = await linkAlertSession(activeAlert.value.id);
    activeAlert.value = result.data.alert;
    alertRecords.value = result.data.records;
    ElMessage.success("已关联排障会话");
    await loadAlerts();

    if (result.data.alert.linked_session_id) {
      router.push({
        path: "/chat",
        query: {
          sessionId: result.data.alert.linked_session_id,
        },
      });
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "会话关联失败");
  } finally {
    linkingSession.value = false;
  }
}

function statusTagType(status: AlertItem["status"]) {
  if (status === "resolved") {
    return "success";
  }
  if (status === "investigating") {
    return "warning";
  }
  if (status === "acknowledged") {
    return "info";
  }
  return "danger";
}

function severityTagType(severity: AlertItem["severity"]) {
  if (severity === "P0" || severity === "P1") {
    return "danger";
  }
  if (severity === "P2") {
    return "warning";
  }
  return "info";
}

function formatTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", {
    hour12: false,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
</script>

<style scoped>
.alerts-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.alerts-toolbar,
.alerts-table-card,
.alerts-detail-card {
  padding: 20px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.08);
}

.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-heading h2,
.detail-header h3,
.timeline-block h4 {
  margin: 0;
}

.section-heading p,
.detail-header p {
  margin: 6px 0 0;
  color: #64748b;
}

.filter-row {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.alerts-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(380px, 0.9fr);
  gap: 20px;
}

.detail-header,
.status-panel {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.detail-descriptions {
  margin-top: 20px;
}

.status-panel {
  margin-top: 20px;
}

.status-panel :deep(.el-select) {
  width: 180px;
}

.timeline-block {
  margin-top: 24px;
}

.timeline-block small {
  color: #64748b;
}

@media (max-width: 1200px) {
  .alerts-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .section-heading,
  .filter-row,
  .detail-header,
  .status-panel {
    flex-direction: column;
    align-items: flex-start;
  }

  .filter-row :deep(.el-select),
  .filter-row :deep(.el-input),
  .status-panel :deep(.el-select),
  .status-panel :deep(.el-input) {
    width: 100%;
  }
}
</style>
