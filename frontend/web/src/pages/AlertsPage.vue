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
        <el-input v-model="filters.query" placeholder="按标题/摘要搜索" @keyup.enter="loadAlerts" />
      </div>

      <div class="stats-row">
        <article class="stat-card">
          <strong>{{ stats.total }}</strong>
          <span>总告警</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.open }}</strong>
          <span>待处理</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.investigating }}</strong>
          <span>排查中</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.linked_sessions }}</strong>
          <span>已关联会话</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.deduplicated_hit }}</strong>
          <span>去重合并次数</span>
        </article>
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
          <el-table-column label="次数" width="90">
            <template #default="{ row }">
              {{ row.occurrence_count }}
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
              <el-button plain :loading="analyzingAlert" @click="handleAnalyzeAlert">
                {{ alertAnalysis ? "刷新 AI 分析" : "生成 AI 分析" }}
              </el-button>
              <el-button type="primary" plain :disabled="linkingSession" @click="handleLinkSession">
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
            <el-descriptions-item label="最近触发">{{ formatTime(activeAlert.last_triggered_at) }}</el-descriptions-item>
            <el-descriptions-item label="触发次数">{{ activeAlert.occurrence_count }}</el-descriptions-item>
            <el-descriptions-item label="关联会话" :span="2">
              {{ activeAlert.linked_session_id || "尚未关联" }}
            </el-descriptions-item>
            <el-descriptions-item label="描述" :span="2">
              {{ activeAlert.description || "暂无详细描述" }}
            </el-descriptions-item>
            <el-descriptions-item label="标签" :span="2">
              <div v-if="labelEntries.length" class="labels-list">
                <el-tag v-for="[key, value] in labelEntries" :key="key" size="small">
                  {{ key }}={{ value }}
                </el-tag>
              </div>
              <span v-else>暂无标签</span>
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

          <div class="analysis-block">
            <div class="analysis-header">
              <h4>AI 分析</h4>
              <div v-if="alertAnalysis" class="analysis-meta">
                <el-tag size="small" :type="analysisTagType(alertAnalysis.status)">
                  {{ alertAnalysis.status }}
                </el-tag>
                <span>{{ formatTime(alertAnalysis.generated_at) }}</span>
                <span>{{ alertAnalysis.source }}</span>
              </div>
            </div>

            <div v-if="alertAnalysis" class="analysis-card">
              <p class="analysis-summary">{{ alertAnalysis.summary }}</p>
              <p class="analysis-assessment">{{ alertAnalysis.severity_assessment }}</p>

              <div class="analysis-diagnostics">
                <div class="message-meta">
                  <span v-if="alertAnalysis.workflow" class="route-chip">Workflow: {{ alertAnalysis.workflow }}</span>
                  <span v-if="alertAnalysis.source" class="route-chip">Source: {{ alertAnalysis.source }}</span>
                  <span v-if="alertAnalysis.tool_calls?.length" class="meta-count">Tools: {{ alertAnalysis.tool_calls.length }}</span>
                  <span v-if="alertAnalysis.trace?.length" class="meta-count">Trace: {{ alertAnalysis.trace.length }}</span>
                </div>

                <div v-if="alertAnalysis.tool_calls?.length" class="tool-call-list">
                  <div
                    v-for="toolCall in alertAnalysis.tool_calls"
                    :key="`${toolCall.name}-${toolCall.arguments_json || toolCall.summary}`"
                    class="tool-call-card"
                  >
                    <header>
                      <strong>{{ toolCall.name }}</strong>
                      <span>{{ toolCall.outcome || "unknown" }}</span>
                    </header>
                    <p v-if="toolCall.summary">{{ toolCall.summary }}</p>
                    <pre v-if="toolCall.arguments_json">{{ toolCall.arguments_json }}</pre>
                  </div>
                </div>

                <div v-else-if="alertAnalysis.recommended_tools.length" class="tool-call-list">
                  <div
                    v-for="tool in alertAnalysis.recommended_tools"
                    :key="tool"
                    class="tool-call-card"
                  >
                    <header>
                      <strong>{{ tool }}</strong>
                      <span>recommended</span>
                    </header>
                    <p>{{ toolSummary(tool) }}</p>
                  </div>
                </div>

                <div v-if="alertAnalysis.trace?.length" class="trace-list">
                  <div
                    v-for="(trace, index) in alertAnalysis.trace"
                    :key="`${activeAlert?.id || 'alert'}-${trace.stage}-${index}`"
                    class="trace-item"
                  >
                    <div class="trace-stage">
                      <strong>{{ trace.stage }}</strong>
                      <span>{{ trace.severity || "info" }}</span>
                    </div>
                    <p>{{ trace.message }}</p>
                    <small v-if="trace.timestamp || trace.tags?.length">
                      {{ [trace.timestamp ? formatTraceTime(trace.timestamp) : "", trace.tags?.length ? trace.tags.join(" · ") : ""].filter(Boolean).join(" · ") }}
                    </small>
                  </div>
                </div>
              </div>

              <div class="analysis-grid">
                <div>
                  <h5>可能原因</h5>
                  <ul>
                    <li v-for="cause in alertAnalysis.possible_causes" :key="cause">{{ cause }}</li>
                  </ul>
                </div>
                <div>
                  <h5>建议动作</h5>
                  <ul>
                    <li v-for="action in alertAnalysis.suggested_actions" :key="action">{{ action }}</li>
                  </ul>
                </div>
              </div>

              <div class="analysis-tags">
                <div>
                  <h5>推荐工具</h5>
                  <div class="labels-list">
                    <el-tag v-for="tool in alertAnalysis.recommended_tools" :key="tool" size="small">
                      {{ tool }}
                    </el-tag>
                  </div>
                </div>
                <div>
                  <h5>知识检索建议</h5>
                  <div class="labels-list">
                    <el-tag v-for="query in alertAnalysis.knowledge_queries" :key="query" size="small" type="info">
                      {{ query }}
                    </el-tag>
                  </div>
                </div>
              </div>

              <p class="analysis-footer">
                工作流：{{ alertAnalysis.workflow || "-" }} · 置信度：{{ alertAnalysis.confidence || "-" }}
              </p>
              <p v-if="alertAnalysis.error" class="analysis-error">{{ alertAnalysis.error }}</p>
            </div>

            <el-empty v-else description="当前还没有 AI 分析结果，点击上方按钮生成。" />
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
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useRouter } from "vue-router";

import {
  analyzeAlert,
  fetchAlertDetail,
  fetchAlerts,
  linkAlertSession,
  updateAlertStatus,
  type AlertAnalysis,
  type AlertItem,
  type AlertRecord,
  type AlertStats,
} from "@/services/api";

const router = useRouter();
const alerts = ref<AlertItem[]>([]);
const activeAlert = ref<AlertItem | null>(null);
const alertRecords = ref<AlertRecord[]>([]);
const stats = ref<AlertStats>({
  total: 0,
  open: 0,
  investigating: 0,
  resolved: 0,
  by_severity: { P0: 0, P1: 0, P2: 0, P3: 0 },
  linked_sessions: 0,
  deduplicated_hit: 0,
});
const updatingStatus = ref(false);
const linkingSession = ref(false);
const analyzingAlert = ref(false);
const nextStatus = ref("");
const statusComment = ref("");
const labelEntries = computed(() => Object.entries(activeAlert.value?.labels || {}));
const alertAnalysis = computed<AlertAnalysis | null>(() => activeAlert.value?.analysis || null);

const filters = reactive({
  status: "",
  severity: "",
  service: "",
  query: "",
});

onMounted(async () => {
  await loadAlerts();
});

async function loadAlerts() {
  const result = await fetchAlerts({
    status: filters.status || undefined,
    severity: filters.severity || undefined,
    service: filters.service.trim() || undefined,
    query: filters.query.trim() || undefined,
  });
  alerts.value = result.data.alerts;
  stats.value = result.data.stats;

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

async function handleAnalyzeAlert() {
  if (!activeAlert.value) {
    return;
  }

  analyzingAlert.value = true;
  try {
    const result = await analyzeAlert(activeAlert.value.id);
    activeAlert.value = result.data.alert;
    alertRecords.value = result.data.records;
    ElMessage.success("AI 分析已更新");
    await loadAlerts();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "AI 分析失败");
  } finally {
    analyzingAlert.value = false;
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

function analysisTagType(status: AlertAnalysis["status"]) {
  if (status === "ready") {
    return "success";
  }
  if (status === "stale") {
    return "warning";
  }
  return "danger";
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

function formatTraceTime(value: string) {
  return formatTime(value);
}

function toolSummary(tool: string) {
  if (tool === "service_status") {
    return "检查服务健康、环境状态和当前风险等级。";
  }
  if (tool === "recent_alerts") {
    return "回看最近相关告警，判断是否为同一波故障或持续抖动。";
  }
  if (tool === "knowledge_search") {
    return "检索 SOP、runbook 和历史案例，补足处置上下文。";
  }
  if (tool === "platform_overview") {
    return "查看平台级风险面，确认是否存在更广的基础设施异常。";
  }
  return "结合当前分析结果继续补充诊断上下文。";
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

.stats-row {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.stat-card {
  padding: 14px 16px;
  border-radius: 16px;
  background: rgba(248, 250, 252, 0.92);
}

.stat-card strong {
  display: block;
  font-size: 24px;
}

.stat-card span {
  color: #64748b;
  font-size: 13px;
}

.alerts-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(380px, 0.9fr);
  gap: 20px;
}

.detail-header,
.status-panel,
.analysis-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.detail-actions {
  display: flex;
  gap: 12px;
}

.detail-descriptions {
  margin-top: 20px;
}

.labels-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-panel {
  margin-top: 20px;
}

.status-panel :deep(.el-select) {
  width: 180px;
}

.analysis-block {
  margin-top: 24px;
  padding: 18px;
  border-radius: 18px;
  background: rgba(248, 250, 252, 0.92);
}

.analysis-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.analysis-diagnostics {
  display: grid;
  gap: 12px;
}

.analysis-header {
  margin-bottom: 14px;
}

.analysis-header h4 {
  margin: 0;
}

.analysis-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  color: #64748b;
  font-size: 12px;
}

.analysis-summary,
.analysis-assessment,
.analysis-footer,
.analysis-error {
  margin: 0;
}

.message-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.route-chip,
.meta-count {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.06);
  color: #334155;
  font-size: 12px;
}

.tool-call-list,
.trace-list {
  display: grid;
  gap: 10px;
}

.tool-call-card,
.trace-item {
  padding: 12px;
  border-radius: 14px;
  background: rgba(241, 245, 249, 0.9);
}

.tool-call-card header,
.trace-stage {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tool-call-card p,
.trace-item p {
  margin: 8px 0 0;
  font-size: 13px;
  color: #334155;
}

.tool-call-card pre {
  margin: 10px 0 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.9);
  color: #e2e8f0;
  overflow-x: auto;
  font-size: 12px;
  line-height: 1.5;
}

.trace-item small {
  display: block;
  margin-top: 8px;
  color: #64748b;
}

.analysis-assessment,
.analysis-footer {
  color: #475569;
}

.analysis-error {
  color: #dc2626;
}

.analysis-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.analysis-grid h5,
.analysis-tags h5 {
  margin: 0 0 10px;
}

.analysis-grid ul {
  margin: 0;
  padding-left: 18px;
  color: #334155;
  line-height: 1.7;
}

.analysis-tags {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.timeline-block {
  margin-top: 24px;
}

.timeline-block small {
  color: #64748b;
}

@media (max-width: 1200px) {
  .stats-row,
  .alerts-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .section-heading,
  .filter-row,
  .detail-header,
  .status-panel,
  .analysis-header,
  .detail-actions {
    flex-direction: column;
    align-items: flex-start;
  }

  .analysis-grid,
  .analysis-tags {
    grid-template-columns: 1fr;
  }

  .filter-row :deep(.el-select),
  .filter-row :deep(.el-input),
  .status-panel :deep(.el-select),
  .status-panel :deep(.el-input) {
    width: 100%;
  }
}
</style>
