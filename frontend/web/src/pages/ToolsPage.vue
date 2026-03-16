<template>
  <div class="tools-page">
    <section class="tools-sidebar">
      <div class="section-heading">
        <div>
          <h2>工具目录</h2>
          <p>统一查看工具定义、权限要求和可执行入口。</p>
        </div>
        <el-button plain @click="loadTools">刷新</el-button>
      </div>

      <div class="tool-filters">
        <el-input
          v-model="toolFilters.query"
          clearable
          placeholder="按工具名搜索"
        />
        <el-select v-model="toolFilters.category" clearable placeholder="全部分类">
          <el-option
            v-for="category in categories"
            :key="category"
            :label="category"
            :value="category"
          />
        </el-select>
      </div>

      <div v-if="filteredTools.length" class="tool-list">
        <button
          v-for="tool in filteredTools"
          :key="tool.name"
          type="button"
          :class="['tool-item', { active: tool.name === selectedToolName }]"
          @click="selectTool(tool.name)"
        >
          <div class="tool-item__header">
            <strong>{{ tool.display_name }}</strong>
            <el-tag size="small" :type="tool.available ? 'success' : 'info'">
              {{ tool.category }}
            </el-tag>
          </div>
          <p>{{ tool.description }}</p>
          <small>角色：{{ tool.allowed_roles.join(" / ") }}</small>
          <small v-if="!tool.available" class="tool-item__warning">{{ tool.unavailable_reason }}</small>
        </button>
      </div>

      <el-empty v-else description="当前没有可用工具定义。" />
    </section>

    <section class="tool-executor">
      <div class="section-heading">
        <div>
          <h2>{{ selectedTool?.display_name || "未选择工具" }}</h2>
          <p>{{ selectedTool?.description || "从左侧选择一个工具开始执行。" }}</p>
        </div>
        <el-tag v-if="selectedTool" :type="selectedTool.available ? 'success' : 'warning'">
          {{ selectedTool.available ? "可调用" : "受限" }}
        </el-tag>
      </div>

      <template v-if="selectedTool">
        <div class="tool-meta">
          <span>分类：{{ selectedTool.category }}</span>
          <span>允许角色：{{ selectedTool.allowed_roles.join(" / ") }}</span>
          <span>当前账号：{{ authStore.user?.display_name || "-" }}</span>
        </div>

        <div class="tool-form">
          <div
            v-for="parameter in selectedTool.parameters"
            :key="parameter.name"
            class="tool-form__field"
          >
            <label>{{ parameter.name }}<em v-if="parameter.required">*</em></label>
            <el-input-number
              v-if="parameter.type === 'number'"
              :model-value="numberFieldValue(parameter.name)"
              :min="1"
              :step="1"
              controls-position="right"
              @update:model-value="updateNumberField(parameter.name, $event)"
            />
            <el-input
              v-else
              :model-value="stringFieldValue(parameter.name)"
              :placeholder="parameter.description"
              @update:model-value="updateStringField(parameter.name, $event)"
            />
            <small>{{ parameter.description }}</small>
          </div>
        </div>

        <div class="tool-actions">
          <el-button
            type="primary"
            :disabled="!selectedTool.available"
            :loading="calling"
            @click="handleCallTool"
          >
            执行工具
          </el-button>
          <el-button plain @click="loadLogs">刷新调用记录</el-button>
        </div>

        <div class="tool-result">
          <div class="tool-result__header">
            <h3>调用结果</h3>
            <small v-if="selectedTool.available">统一协议返回，便于后续接入 Agent。</small>
          </div>
          <div v-if="lastCallMeta" class="tool-result__meta">
            <span>执行状态：{{ lastCallMeta.status }}</span>
            <span>耗时：{{ lastCallMeta.duration_ms }} ms</span>
            <span>时间：{{ formatTime(lastCallMeta.created_at) }}</span>
          </div>
          <el-empty v-if="!callResult" description="执行工具后，这里会展示返回结果。" />
          <pre v-else>{{ formattedResult }}</pre>
        </div>
      </template>
    </section>

    <section class="tool-logs">
      <div class="section-heading">
        <div>
          <h2>调用记录</h2>
          <p>查看工具执行结果、耗时与失败原因。</p>
        </div>
        <div class="log-filters">
          <el-select v-model="logFilters.status" clearable placeholder="全部状态" @change="loadLogs">
            <el-option label="success" value="success" />
            <el-option label="failed" value="failed" />
            <el-option label="forbidden" value="forbidden" />
          </el-select>
          <el-button plain @click="loadLogs">刷新记录</el-button>
        </div>
      </div>

      <el-table :data="logs" stripe>
        <el-table-column prop="tool_name" label="工具" min-width="180" />
        <el-table-column prop="operator" label="调用人" width="140" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="duration_ms" label="耗时(ms)" width="120" />
        <el-table-column label="时间" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="输入 / 错误" min-width="340">
          <template #default="{ row }">
            <div class="log-cell">
              <code>{{ stringify(row.input) }}</code>
              <span v-if="row.error" class="log-error">{{ row.error }}</span>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";

import {
  callTool,
  fetchToolLogs,
  fetchTools,
  type ToolCallLog,
  type ToolDefinition,
} from "@/services/api";
import { useAuthStore } from "@/stores/auth";

const authStore = useAuthStore();
const tools = ref<ToolDefinition[]>([]);
const selectedToolName = ref("");
const fieldValues = ref<Record<string, string | number>>({});
const callResult = ref<unknown>(null);
const logs = ref<ToolCallLog[]>([]);
const calling = ref(false);
const toolFilters = ref({
  query: "",
  category: "",
});
const logFilters = ref({
  status: "",
});

const selectedTool = computed(
  () => tools.value.find((tool) => tool.name === selectedToolName.value) || null
);
const filteredTools = computed(() =>
  tools.value.filter((tool) => {
    if (toolFilters.value.category && tool.category !== toolFilters.value.category) {
      return false;
    }
    if (!toolFilters.value.query.trim()) {
      return true;
    }
    const query = toolFilters.value.query.trim().toLowerCase();
    return (
      tool.display_name.toLowerCase().includes(query) ||
      tool.name.toLowerCase().includes(query) ||
      tool.description.toLowerCase().includes(query)
    );
  })
);
const categories = computed(() => Array.from(new Set(tools.value.map((tool) => tool.category))));
const formattedResult = computed(() => JSON.stringify(callResult.value, null, 2));
const lastCallMeta = computed(() => logs.value[0] || null);

onMounted(async () => {
  await loadTools();
  await loadLogs();
});

watch(selectedTool, (tool) => {
  if (!tool) {
    fieldValues.value = {};
    return;
  }

  const nextValues: Record<string, string | number> = {};
  for (const parameter of tool.parameters) {
    if (parameter.default !== undefined) {
      nextValues[parameter.name] = parameter.default;
      continue;
    }
    nextValues[parameter.name] = parameter.type === "number" ? 1 : "";
  }
  fieldValues.value = nextValues;
  callResult.value = null;
});

async function loadTools() {
  const result = await fetchTools();
  tools.value = result.data.tools;

  if (!selectedToolName.value && tools.value.length > 0) {
    selectedToolName.value = tools.value.find((tool) => tool.available)?.name || tools.value[0].name;
  }
}

async function loadLogs() {
  const result = await fetchToolLogs({
    limit: 12,
    tool_name: selectedToolName.value || undefined,
    status: logFilters.value.status || undefined,
  });
  logs.value = result.data.logs;
}

function selectTool(toolName: string) {
  selectedToolName.value = toolName;
  void loadLogs();
}

async function handleCallTool() {
  if (!selectedTool.value) {
    return;
  }

  calling.value = true;
  try {
    const result = await callTool(selectedTool.value.name, fieldValues.value);
    callResult.value = result.data.result;
    ElMessage.success("工具调用完成");
    await loadLogs();
  } catch (error) {
    callResult.value = null;
    ElMessage.error(error instanceof Error ? error.message : "工具调用失败");
  } finally {
    calling.value = false;
  }
}

function stringFieldValue(name: string) {
  return String(fieldValues.value[name] ?? "");
}

function numberFieldValue(name: string) {
  return Number(fieldValues.value[name] ?? 1);
}

function updateStringField(name: string, value: string) {
  fieldValues.value = {
    ...fieldValues.value,
    [name]: value,
  };
}

function updateNumberField(name: string, value: number | null | undefined) {
  fieldValues.value = {
    ...fieldValues.value,
    [name]: value ?? 1,
  };
}

function stringify(value: unknown) {
  return JSON.stringify(value);
}

function formatTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", {
    hour12: false,
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
</script>

<style scoped>
.tools-page {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
}

.tools-sidebar,
.tool-executor,
.tool-logs {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.08);
}

.tools-sidebar,
.tool-executor {
  padding: 20px;
}

.tool-logs {
  grid-column: 1 / -1;
  padding: 20px;
}

.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-heading h2,
.tool-result__header h3 {
  margin: 0;
  font-size: 20px;
}

.section-heading p,
.tool-result__header small {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 13px;
}

.tool-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 20px;
}

.tool-filters,
.log-filters,
.tool-result__meta {
  display: flex;
  gap: 12px;
}

.tool-filters {
  margin-top: 16px;
}

.tool-item {
  width: 100%;
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 16px;
  padding: 14px 16px;
  background: #fff;
  text-align: left;
  cursor: pointer;
}

.tool-item.active {
  border-color: rgba(14, 116, 144, 0.45);
  background: rgba(240, 249, 255, 0.9);
}

.tool-item__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tool-item p,
.tool-item small,
.tool-meta span,
.tool-form__field small {
  color: #64748b;
}

.tool-item p {
  margin: 10px 0;
  line-height: 1.6;
}

.tool-item small {
  display: block;
  font-size: 12px;
}

.tool-item__warning {
  margin-top: 6px;
  color: #b45309;
}

.tool-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  margin-top: 18px;
  font-size: 13px;
}

.tool-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-top: 20px;
}

.tool-form__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tool-form__field label {
  font-size: 14px;
  font-weight: 600;
}

.tool-form__field em {
  margin-left: 4px;
  color: #dc2626;
  font-style: normal;
}

.tool-actions {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.tool-result {
  margin-top: 20px;
  padding: 18px;
  border-radius: 18px;
  background: rgba(248, 250, 252, 0.92);
}

.tool-result__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.tool-result__meta {
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.tool-result__meta span {
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(226, 232, 240, 0.75);
  color: #475569;
  font-size: 12px;
}

.tool-result pre {
  margin: 0;
  padding: 16px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.96);
  color: #e2e8f0;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.log-cell code {
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-error {
  color: #dc2626;
}

@media (max-width: 1100px) {
  .tools-page {
    grid-template-columns: 1fr;
  }

  .tool-filters,
  .log-filters {
    flex-direction: column;
    align-items: stretch;
  }

  .tool-form {
    grid-template-columns: 1fr;
  }
}
</style>
