<template>
  <div class="chat-workspace">
    <aside class="chat-sidebar">
      <div class="sidebar-header">
        <div>
          <h2>会话列表</h2>
          <p>阶段 5 先跑通会话、消息历史和 SSE 流式回复。</p>
        </div>
        <el-button type="primary" @click="handleCreateSession">新建会话</el-button>
      </div>

      <div v-if="sessions.length" class="session-list">
        <div
          v-for="session in sessions"
          :key="session.id"
          :class="['session-item', { active: session.id === activeSessionId }]"
          @click="selectSession(session.id)"
        >
          <div>
            <strong>{{ session.title }}</strong>
            <small>{{ formatTime(session.updated_at) }}</small>
          </div>
          <el-button text type="danger" @click.stop="handleDeleteSession(session.id)">删除</el-button>
        </div>
      </div>

      <el-empty v-else description="还没有会话，先创建一个会话开始对话。" />
    </aside>

    <section class="chat-panel">
      <div class="chat-panel-header">
        <div>
          <h2>{{ activeSession?.title || "未选择会话" }}</h2>
          <p v-if="activeSession">当前会话消息会保存在 Go 服务内存中，可立即查询历史。</p>
          <p v-else>选择左侧会话，或者先创建一个新的会话。</p>
        </div>
      </div>

      <div ref="messageListRef" class="message-list">
        <el-empty v-if="!messages.length" description="发送第一条消息后，这里会显示完整的会话历史。" />

        <article
          v-for="message in messages"
          :key="message.id"
          :class="['message-bubble', message.role]"
        >
          <header>
            <strong>{{ message.role === "user" ? "你" : "AI OnCall" }}</strong>
            <span>{{ formatTime(message.created_at) }}</span>
          </header>
          <p>{{ message.content || (message.status === 'streaming' ? '正在生成回复...' : '') }}</p>
        </article>
      </div>

      <div class="composer">
        <el-input
          v-model="draft"
          type="textarea"
          :rows="4"
          resize="none"
          :disabled="sending"
          placeholder="输入你的排障问题、告警上下文或要查询的系统信息"
        />
        <div class="composer-actions">
          <span>演示阶段先返回占位流式回复，下一阶段接 AI 编排。</span>
          <el-button type="primary" :loading="sending" @click="handleSend">发送消息</el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";

import {
  createSession,
  deleteSession,
  fetchSessionMessages,
  fetchSessions,
  streamSessionMessage,
  type ChatMessage,
  type ChatSession,
} from "@/services/api";

const sessions = ref<ChatSession[]>([]);
const activeSessionId = ref("");
const messages = ref<ChatMessage[]>([]);
const draft = ref("");
const sending = ref(false);
const messageListRef = ref<HTMLElement | null>(null);

const activeSession = computed(() =>
  sessions.value.find((session) => session.id === activeSessionId.value) || null
);

onMounted(async () => {
  await loadSessions();
});

async function loadSessions() {
  const result = await fetchSessions();
  sessions.value = result.data.sessions;
  if (!activeSessionId.value && sessions.value.length > 0) {
    await selectSession(sessions.value[0].id);
  }
}

async function selectSession(sessionId: string) {
  activeSessionId.value = sessionId;
  const result = await fetchSessionMessages(sessionId);
  messages.value = result.data.messages;
  await scrollToBottom();
}

async function handleCreateSession() {
  const result = await createSession();
  const session = result.data.session;
  sessions.value = [session, ...sessions.value];
  activeSessionId.value = session.id;
  messages.value = [];
}

async function ensureActiveSession() {
  if (activeSessionId.value) {
    return activeSessionId.value;
  }

  const result = await createSession();
  const session = result.data.session;
  sessions.value = [session, ...sessions.value];
  activeSessionId.value = session.id;
  return session.id;
}

async function handleDeleteSession(sessionId: string) {
  await deleteSession(sessionId);
  sessions.value = sessions.value.filter((session) => session.id !== sessionId);

  if (activeSessionId.value !== sessionId) {
    return;
  }

  activeSessionId.value = sessions.value[0]?.id || "";
  if (activeSessionId.value) {
    await selectSession(activeSessionId.value);
  } else {
    messages.value = [];
  }
}

async function handleSend() {
  const content = draft.value.trim();
  if (!content || sending.value) {
    return;
  }

  const sessionId = await ensureActiveSession();
  const now = new Date().toISOString();
  const userMessage: ChatMessage = {
    id: `local-user-${Date.now()}`,
    session_id: sessionId,
    role: "user",
    content,
    status: "completed",
    created_at: now,
  };
  const assistantMessage: ChatMessage = {
    id: `local-assistant-${Date.now()}`,
    session_id: sessionId,
    role: "assistant",
    content: "",
    status: "streaming",
    created_at: now,
  };

  messages.value = [...messages.value, userMessage, assistantMessage];
  draft.value = "";
  sending.value = true;
  await scrollToBottom();

  try {
    await streamSessionMessage(sessionId, content, {
      onChunk: async ({ message_id, delta }) => {
        const target = messages.value.find((item) => item.id === assistantMessage.id || item.id === message_id);
        if (!target) {
          return;
        }

        target.id = message_id;
        target.content += delta;
        target.status = "streaming";
        await scrollToBottom();
      },
      onDone: ({ message_id, content: finalContent }) => {
        const target = messages.value.find((item) => item.id === assistantMessage.id || item.id === message_id);
        if (!target) {
          return;
        }

        target.id = message_id;
        target.content = finalContent;
        target.status = "completed";
      },
    });

    await loadSessions();
    await selectSession(sessionId);
  } catch (error) {
    assistantMessage.status = "failed";
    assistantMessage.content = "消息发送失败，请稍后重试。";
    ElMessage.error(error instanceof Error ? error.message : "消息发送失败");
  } finally {
    sending.value = false;
    await scrollToBottom();
  }
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

async function scrollToBottom() {
  await nextTick();
  if (!messageListRef.value) {
    return;
  }
  messageListRef.value.scrollTop = messageListRef.value.scrollHeight;
}
</script>

<style scoped>
.chat-workspace {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
  min-height: calc(100vh - 220px);
}

.chat-sidebar,
.chat-panel {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.08);
}

.chat-sidebar {
  padding: 20px;
}

.sidebar-header,
.chat-panel-header,
.composer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.sidebar-header h2,
.chat-panel-header h2 {
  margin: 0;
  font-size: 20px;
}

.sidebar-header p,
.chat-panel-header p,
.composer-actions span {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 13px;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 20px;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 14px 16px;
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 16px;
  background: #fff;
  text-align: left;
  cursor: pointer;
}

.session-item.active {
  border-color: rgba(14, 116, 144, 0.45);
  background: rgba(240, 249, 255, 0.9);
}

.session-item strong,
.message-bubble strong {
  display: block;
}

.session-item small,
.message-bubble span {
  color: #64748b;
  font-size: 12px;
}

.chat-panel {
  display: flex;
  flex-direction: column;
  padding: 20px;
}

.message-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 420px;
  max-height: 620px;
  margin: 20px 0;
  padding: 8px 4px 8px 0;
  overflow: auto;
}

.message-bubble {
  max-width: 78%;
  padding: 16px 18px;
  border-radius: 18px;
  background: #f8fafc;
}

.message-bubble.user {
  margin-left: auto;
  background: #dbeafe;
}

.message-bubble.assistant {
  margin-right: auto;
  background: #f8fafc;
}

.message-bubble header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 8px;
}

.message-bubble p {
  margin: 0;
  color: #0f172a;
  line-height: 1.7;
  white-space: pre-wrap;
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

@media (max-width: 1100px) {
  .chat-workspace {
    grid-template-columns: 1fr;
  }

  .message-bubble {
    max-width: 100%;
  }

  .sidebar-header,
  .chat-panel-header,
  .composer-actions {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
