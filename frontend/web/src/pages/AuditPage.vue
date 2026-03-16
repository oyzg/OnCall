<template>
  <div class="audit-page">
    <section class="audit-overview">
      <div class="section-heading">
        <div>
          <h2>审计与运营</h2>
          <p>查看核心操作留痕、失败情况和最近 24 小时活跃度。</p>
        </div>
        <el-button plain @click="loadAll">刷新</el-button>
      </div>

      <div class="stats-row">
        <article class="stat-card">
          <strong>{{ stats.total }}</strong>
          <span>总事件</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.success }}</strong>
          <span>成功事件</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.failed }}</strong>
          <span>失败事件</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.last_24_hours }}</strong>
          <span>24 小时新增</span>
        </article>
        <article class="stat-card">
          <strong>{{ stats.unique_actors }}</strong>
          <span>活跃账号</span>
        </article>
      </div>

      <div class="category-row">
        <article v-for="entry in categoryEntries" :key="entry[0]" class="category-card">
          <strong>{{ entry[1] }}</strong>
          <span>{{ entry[0] }}</span>
        </article>
      </div>
    </section>

    <section class="audit-table-card">
      <div class="filter-row">
        <el-select v-model="filters.category" clearable placeholder="全部分类" @change="loadLogs">
          <el-option v-for="category in categories" :key="category" :label="category" :value="category" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="全部状态" @change="loadLogs">
          <el-option label="success" value="success" />
          <el-option label="failed" value="failed" />
        </el-select>
        <el-input v-model="filters.actor" placeholder="按操作人筛选" @keyup.enter="loadLogs" />
        <el-input v-model="filters.action" placeholder="按动作筛选" @keyup.enter="loadLogs" />
      </div>

      <el-table :data="logs" stripe>
        <el-table-column prop="category" label="分类" width="120" />
        <el-table-column prop="action" label="动作" width="140" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作人" min-width="160">
          <template #default="{ row }">
            {{ row.actor_name || row.actor_id || "-" }}
          </template>
        </el-table-column>
        <el-table-column label="目标" min-width="220">
          <template #default="{ row }">
            {{ row.target_type || "-" }} / {{ row.target_name || row.target_id || "-" }}
          </template>
        </el-table-column>
        <el-table-column prop="detail" label="说明" min-width="240" />
        <el-table-column label="时间" width="190">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="元数据" min-width="260">
          <template #default="{ row }">
            <code>{{ stringify(row.metadata) }}</code>
          </template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";

import { fetchAuditLogs, fetchAuditStats, type AuditLog, type AuditStats } from "@/services/api";

const logs = ref<AuditLog[]>([]);
const categories = ref<string[]>([]);
const stats = ref<AuditStats>({
  total: 0,
  success: 0,
  failed: 0,
  last_24_hours: 0,
  unique_actors: 0,
  by_category: {},
});

const filters = reactive({
  category: "",
  status: "",
  actor: "",
  action: "",
});

const categoryEntries = computed(() =>
  Object.entries(stats.value.by_category)
    .sort((left, right) => right[1] - left[1])
    .slice(0, 6)
);

onMounted(async () => {
  await loadAll();
});

async function loadAll() {
  await Promise.all([loadLogs(), loadStats()]);
}

async function loadLogs() {
  const result = await fetchAuditLogs({
    category: filters.category || undefined,
    status: filters.status || undefined,
    actor: filters.actor.trim() || undefined,
    action: filters.action.trim() || undefined,
    limit: 100,
  });
  logs.value = result.data.logs;
  categories.value = result.data.categories;
}

async function loadStats() {
  const result = await fetchAuditStats();
  stats.value = result.data.stats;
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

function stringify(value: unknown) {
  return value ? JSON.stringify(value) : "-";
}
</script>

<style scoped>
.audit-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.audit-overview,
.audit-table-card {
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

.section-heading h2 {
  margin: 0;
}

.section-heading p {
  margin: 6px 0 0;
  color: #64748b;
}

.stats-row,
.category-row {
  display: grid;
  gap: 12px;
  margin-top: 18px;
}

.stats-row {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.category-row {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.stat-card,
.category-card {
  padding: 14px 16px;
  border-radius: 16px;
  background: rgba(248, 250, 252, 0.92);
}

.stat-card strong,
.category-card strong {
  display: block;
  font-size: 24px;
}

.stat-card span,
.category-card span {
  color: #64748b;
  font-size: 13px;
}

.filter-row {
  display: flex;
  gap: 12px;
  margin-bottom: 18px;
}

code {
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 1200px) {
  .stats-row,
  .category-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .section-heading,
  .filter-row {
    flex-direction: column;
    align-items: flex-start;
  }

  .filter-row :deep(.el-select),
  .filter-row :deep(.el-input) {
    width: 100%;
  }
}
</style>
