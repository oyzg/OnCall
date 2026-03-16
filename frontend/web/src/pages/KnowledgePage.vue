<template>
  <div class="knowledge-page">
    <section class="knowledge-upload">
      <div class="section-heading">
        <div>
          <h2>上传知识</h2>
          <p>阶段 6 先完成知识文档管理闭环，支持文件上传和文本录入两种方式。</p>
        </div>
        <el-button type="primary" :loading="uploading" @click="handleUpload">上传到知识库</el-button>
      </div>

      <div class="upload-grid">
        <el-card shadow="never">
          <template #header>文档信息</template>
          <el-form label-position="top">
            <el-form-item label="标题">
              <el-input v-model="form.title" placeholder="例如：支付服务值班手册" />
            </el-form-item>
            <el-form-item label="分类">
              <el-select v-model="form.category" placeholder="选择分类">
                <el-option
                  v-for="option in categories"
                  :key="option"
                  :label="option"
                  :value="option"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="上传文件">
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                :on-change="handleFileChange"
                :limit="1"
              >
                <el-button plain>选择文件</el-button>
              </el-upload>
              <p v-if="selectedFile" class="file-hint">
                已选择：{{ selectedFile.name }} ({{ formatSize(selectedFile.size) }})
              </p>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never">
          <template #header>文本录入</template>
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="11"
            resize="none"
            placeholder="如果不上传文件，也可以直接输入知识文本。"
          />
        </el-card>
      </div>
    </section>

    <section class="knowledge-list-panel">
      <div class="section-heading">
        <div>
          <h2>知识文档</h2>
          <p>查看文档状态、处理结果和生命周期。</p>
        </div>
        <el-button plain @click="loadDocuments">刷新</el-button>
      </div>

      <div class="knowledge-table">
        <el-table :data="documents" stripe>
          <el-table-column prop="title" label="标题" min-width="220" />
          <el-table-column prop="category" width="145">
            <template #header>
              <el-select
                v-model="filters.category"
                clearable
                size="small"
                placeholder="分类"
                @change="loadDocuments"
              >
                <el-option
                  v-for="option in categories"
                  :key="option"
                  :label="option"
                  :value="option"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column prop="source_type" label="来源" width="110" />
          <el-table-column width="145">
            <template #header>
              <el-select
                v-model="filters.status"
                clearable
                size="small"
                placeholder="状态"
                @change="loadDocuments"
              >
                <el-option label="uploaded" value="uploaded" />
                <el-option label="processing" value="processing" />
                <el-option label="ready" value="ready" />
                <el-option label="failed" value="failed" />
              </el-select>
            </template>
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="大小" width="120">
            <template #default="{ row }">
              {{ formatSize(row.size_bytes) }}
            </template>
          </el-table-column>
          <el-table-column label="更新时间" min-width="170">
            <template #default="{ row }">
              {{ formatTime(row.updated_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button text type="primary" @click="openDetail(row.id)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <el-drawer v-model="detailVisible" title="文档详情" size="520px">
      <template v-if="activeDocument">
        <div class="detail-block">
          <strong>{{ activeDocument.title }}</strong>
          <el-tag :type="statusTagType(activeDocument.status)">{{ activeDocument.status }}</el-tag>
        </div>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="分类">{{ activeDocument.category }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ activeDocument.source_type }}</el-descriptions-item>
          <el-descriptions-item label="文件名">{{ activeDocument.file_name }}</el-descriptions-item>
          <el-descriptions-item label="内容类型">{{ activeDocument.content_type }}</el-descriptions-item>
          <el-descriptions-item label="存储路径">{{ activeDocument.storage_path }}</el-descriptions-item>
          <el-descriptions-item label="大小">{{ formatSize(activeDocument.size_bytes) }}</el-descriptions-item>
          <el-descriptions-item label="切片数">{{ activeDocument.chunk_count }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(activeDocument.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="处理完成">
            {{ activeDocument.processed_at ? formatTime(activeDocument.processed_at) : "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="摘要">{{ activeDocument.summary }}</el-descriptions-item>
          <el-descriptions-item v-if="activeDocument.failure_reason" label="失败原因">
            {{ activeDocument.failure_reason }}
          </el-descriptions-item>
        </el-descriptions>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ElMessage, type UploadFile, type UploadFiles } from "element-plus";

import {
  fetchKnowledgeDocument,
  fetchKnowledgeDocuments,
  uploadKnowledgeDocument,
  type KnowledgeDocument,
} from "@/services/api";

const categories = ["general", "runbook", "faq", "incident", "release"];

const form = reactive({
  title: "",
  category: "general",
  content: "",
});

const filters = reactive({
  status: "",
  category: "",
});

const selectedFile = ref<File | null>(null);
const uploading = ref(false);
const documents = ref<KnowledgeDocument[]>([]);
const detailVisible = ref(false);
const activeDocument = ref<KnowledgeDocument | null>(null);
let refreshTimer: number | null = null;

const shouldPoll = computed(() =>
  documents.value.some((document) => document.status === "uploaded" || document.status === "processing")
);

onMounted(async () => {
  await loadDocuments();
  scheduleRefresh();
});

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearTimeout(refreshTimer);
  }
});

async function loadDocuments() {
  const result = await fetchKnowledgeDocuments({
    status: filters.status || undefined,
    category: filters.category || undefined,
  });
  documents.value = result.data.documents;
  scheduleRefresh();
}

function handleFileChange(file: UploadFile, _files: UploadFiles) {
  selectedFile.value = file.raw || null;
}

async function handleUpload() {
  if (!selectedFile.value && !form.content.trim()) {
    ElMessage.warning("请选择文件，或者直接录入文本内容");
    return;
  }

  uploading.value = true;
  try {
    const result = await uploadKnowledgeDocument({
      title: form.title,
      category: form.category,
      content: form.content,
      file: selectedFile.value,
    });

    ElMessage.success("文档已进入知识库");
    documents.value = [result.data.document, ...documents.value];
    resetForm();
    scheduleRefresh(true);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "上传失败");
  } finally {
    uploading.value = false;
  }
}

async function openDetail(documentId: string) {
  const result = await fetchKnowledgeDocument(documentId);
  activeDocument.value = result.data.document;
  detailVisible.value = true;
}

function resetForm() {
  form.title = "";
  form.category = "general";
  form.content = "";
  selectedFile.value = null;
}

function scheduleRefresh(force = false) {
  if (refreshTimer) {
    window.clearTimeout(refreshTimer);
    refreshTimer = null;
  }

  if (!force && !shouldPoll.value) {
    return;
  }

  refreshTimer = window.setTimeout(async () => {
    await loadDocuments();
  }, 1200);
}

function statusTagType(status: KnowledgeDocument["status"]) {
  if (status === "ready") {
    return "success";
  }
  if (status === "processing") {
    return "warning";
  }
  if (status === "failed") {
    return "danger";
  }
  return "info";
}

function formatTime(value?: string) {
  if (!value) {
    return "-";
  }

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

function formatSize(size: number) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}
</script>

<style scoped>
.knowledge-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.knowledge-upload,
.knowledge-list-panel {
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
  margin-bottom: 20px;
}

.section-heading h2 {
  margin: 0;
  font-size: 20px;
}

.section-heading p,
.file-hint {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 13px;
}

.upload-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.knowledge-table {
  overflow: hidden;
}

.knowledge-table :deep(.el-table th.el-table__cell) {
  background: transparent;
}

.knowledge-table :deep(.el-table__header .el-select) {
  width: 100%;
}

.knowledge-table :deep(.el-table__header .el-select .el-input__wrapper) {
  padding: 0 10px;
}

.knowledge-table :deep(.el-table__header .el-select .el-input__inner) {
  font-size: 13px;
}

.detail-block {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

@media (max-width: 1100px) {
  .upload-grid {
    grid-template-columns: 1fr;
  }

  .section-heading {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
