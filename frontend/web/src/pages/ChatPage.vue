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
        >
          <button class="session-button" type="button" @click="selectSession(session.id)">
            <strong>{{ session.title }}</strong>
            <small>{{ formatTime(session.updated_at) }}</small>
          </button>
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
          <div v-if="message.references?.length" class="reference-list">
            <strong>引用片段</strong>
            <div
              v-for="reference in message.references"
              :key="`${message.id}-${reference.document_id}-${reference.excerpt}`"
              class="reference-card"
            >
              <header>
                <span>{{ reference.document_title }}</span>
                <small>{{ reference.category }} · {{ reference.score.toFixed(2) }}</small>
              </header>
              <p>{{ reference.excerpt }}</p>
            </div>
          </div>
        </article>
      </div>

      <div class="composer">
        <textarea
          ref="composerTextareaRef"
          :value="draft"
          class="composer-textarea"
          :disabled="sending"
          placeholder="输入你的排障问题、告警上下文或要查询的系统信息"
          @click="focusComposer"
          @input="handleDraftInput"
          @keydown.enter.exact.prevent="handleSend"
          @keydown.ctrl.enter.prevent="handleSend"
          @keydown.meta.enter.prevent="handleSend"
        />
        <div class="composer-actions">
          <span>当前会结合知识库返回引用片段，`Enter`、`Ctrl+Enter` / `Cmd+Enter` 都可以发送。</span>
          <button class="send-button" type="button" :disabled="sending" @click="handleSend">
            {{ sending ? "发送中..." : "发送消息" }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { useRoute, useRouter } from "vue-router";

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
const composerTextareaRef = ref<HTMLTextAreaElement | null>(null);
const sessionLoadToken = ref(0);
const route = useRoute();
const router = useRouter();

const activeSession = computed(() =>
  sessions.value.find((session) => session.id === activeSessionId.value) || null
);

onMounted(async () => {
  await loadSessions();
  await focusComposer();
});

async function loadSessions() {
  const result = await fetchSessions();
  sessions.value = result.data.sessions;
  const preferredSessionId = typeof route.query.sessionId === "string" ? route.query.sessionId : "";

  if (preferredSessionId && sessions.value.some((session) => session.id === preferredSessionId)) {
    await selectSession(preferredSessionId);
    await router.replace({ path: "/chat" });
    return;
  }

  if (!activeSessionId.value && sessions.value.length > 0) {
    await selectSession(sessions.value[0].id);
  }
}

async function selectSession(sessionId: string) {
  const currentLoadToken = ++sessionLoadToken.value;
  activeSessionId.value = sessionId;
  messages.value = [];
  const result = await fetchSessionMessages(sessionId);

  if (currentLoadToken !== sessionLoadToken.value || activeSessionId.value !== sessionId) {
    return;
  }

  messages.value = Array.isArray(result.data.messages) ? result.data.messages : [];
  await scrollToBottom();
  await focusComposer();
}

async function handleCreateSession() {
  const result = await createSession();
  const session = result.data.session;
  sessions.value = [session, ...sessions.value];
  activeSessionId.value = session.id;
  messages.value = [];
  await focusComposer();
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
  try {
    const content = (composerTextareaRef.value?.value || draft.value).trim();
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

    const currentMessages = Array.isArray(messages.value) ? messages.value : [];
    messages.value = [...currentMessages, userMessage, assistantMessage];
    draft.value = "";
    if (composerTextareaRef.value) {
      composerTextareaRef.value.value = "";
    }
    sending.value = true;
    await scrollToBottom();

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
      onDone: ({ message_id, content: finalContent, references }) => {
        const target = messages.value.find((item) => item.id === assistantMessage.id || item.id === message_id);
        if (!target) {
          return;
        }

        target.id = message_id;
        target.content = finalContent;
        target.status = "completed";
        target.references = references || [];
      },
    });

    await loadSessions();
    await selectSession(sessionId);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "消息发送失败");
  } finally {
    sending.value = false;
    await scrollToBottom();
    await focusComposer();
  }
}

function handleDraftInput(event: Event) {
  draft.value = (event.target as HTMLTextAreaElement).value;
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

async function focusComposer() {
  await nextTick();
  composerTextareaRef.value?.focus();
}
</script>

<style scoped>
.chat-workspace {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
  height: calc(100vh - 170px);
  min-height: 720px;
}

.chat-sidebar,
.chat-panel {
  min-height: 0;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.08);
}

.chat-sidebar {
  display: flex;
  flex-direction: column;
  padding: 20px;
  overflow: hidden;
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
  min-height: 0;
  overflow: auto;
  padding-right: 4px;
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
}

.session-item.active {
  border-color: rgba(14, 116, 144, 0.45);
  background: rgba(240, 249, 255, 0.9);
}

.session-button {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  border: 0;
  padding: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
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
  overflow: hidden;
}

.message-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
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

.reference-list {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px dashed rgba(148, 163, 184, 0.45);
}

.reference-list > strong {
  display: block;
  margin-bottom: 10px;
  font-size: 13px;
}

.reference-card {
  padding: 12px;
  border-radius: 14px;
  background: rgba(241, 245, 249, 0.9);
}

.reference-card + .reference-card {
  margin-top: 10px;
}

.reference-card header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.reference-card p {
  margin: 0;
  font-size: 13px;
  color: #334155;
}

.composer {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(255, 255, 255, 0.98);
}

.composer-textarea {
  width: 100%;
  min-height: 116px;
  padding: 14px 16px;
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 14px;
  outline: none;
  resize: vertical;
  font: inherit;
  line-height: 1.7;
  color: #0f172a;
  background: rgba(255, 255, 255, 0.96);
}

.composer-textarea:focus {
  border-color: #4f7cff;
  box-shadow: 0 0 0 3px rgba(79, 124, 255, 0.12);
}

.composer-textarea:disabled {
  background: rgba(241, 245, 249, 0.92);
  cursor: not-allowed;
}

.send-button {
  border: 0;
  border-radius: 12px;
  padding: 12px 18px;
  background: #4f7cff;
  color: #fff;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

.send-button:disabled {
  background: rgba(79, 124, 255, 0.55);
  cursor: not-allowed;
}

@media (max-width: 1100px) {
  .chat-workspace {
    grid-template-columns: 1fr;
    height: auto;
    min-height: 0;
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
