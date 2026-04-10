import { http } from "@/services/http";
import { getAccessToken } from "@/stores/session";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://127.0.0.1:8080";

export interface ApiEnvelope<T> {
  code: string;
  message: string;
  request_id?: string;
  data: T;
}

export interface AuthUser {
  id: string;
  username: string;
  display_name: string;
  roles: string[];
}

export interface LoginPayload {
  username: string;
  password: string;
}

export interface LoginResponse {
  access_token: {
    token: string;
    expires_in: number;
  };
  user: AuthUser;
}

export interface ChatSession {
  id: string;
  user_id: string;
  title: string;
  last_message_preview: string;
  message_count: number;
  created_at: string;
  updated_at: string;
  last_message_at: string;
}

export interface ChatMessage {
  id: string;
  session_id: string;
  role: "user" | "assistant";
  content: string;
  status: "streaming" | "completed" | "failed";
  route?: string;
  tool_calls?: ChatToolCall[];
  trace?: ChatTraceEvent[];
  references?: MessageReference[];
  created_at: string;
}

export interface ChatToolCall {
  name: string;
  arguments_json?: string;
  outcome?: string;
  summary?: string;
}

export interface ChatTraceEvent {
  stage: string;
  message: string;
  severity?: string;
  timestamp?: string;
  tags?: string[];
}

export interface ChatMessagePage {
  messages: ChatMessage[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
}

export interface MessageReference {
  document_id: string;
  document_title: string;
  category: string;
  excerpt: string;
  score: number;
}

export interface KnowledgeDocument {
  id: string;
  user_id: string;
  title: string;
  category: string;
  source_type: "file" | "text";
  file_name: string;
  content_type: string;
  storage_path: string;
  size_bytes: number;
  status: "uploaded" | "processing" | "ready" | "failed";
  summary: string;
  text_preview: string;
  chunk_previews: string[];
  failure_reason?: string;
  chunk_count: number;
  index_status?: string;
  embedding_backend?: string;
  vector_backend?: string;
  lexical_backend?: string;
  index_error?: string;
  created_at: string;
  updated_at: string;
  processed_at?: string;
  indexed_at?: string;
}

export interface HealthComponent {
  name: string;
  status: string;
  detail?: string;
}

export interface PythonHealthReport {
  service: string;
  env: string;
  status: string;
  components: HealthComponent[];
}

export interface RetrievalReference {
  document_id: string;
  document_title: string;
  category: string;
  chunk_index?: number;
  chunk: string;
  score: number;
  lexical_score?: number;
  semantic_score?: number;
  boost_score?: number;
  match_reasons?: string[];
}

export interface RetrievalReport {
  query: string;
  rewritten_query: string;
  query_terms: string[];
  expanded_terms: string[];
  answer: string;
  references: RetrievalReference[];
  scanned_docs: number;
  scanned_chunks: number;
  matched_chunks: number;
  lexical_candidates: number;
  semantic_candidates: number;
  reranked_chunks: number;
  strategy: string;
  requested_limit: number;
  embedding_backend?: string;
  vector_backend?: string;
  lexical_backend?: string;
}

export interface AlertItem {
  id: string;
  title: string;
  service: string;
  environment: string;
  severity: "P0" | "P1" | "P2" | "P3";
  source: string;
  status: "open" | "acknowledged" | "investigating" | "resolved";
  summary: string;
  description: string;
  labels?: Record<string, string>;
  linked_session_id?: string;
  analysis?: AlertAnalysis;
  occurrence_count: number;
  created_at: string;
  updated_at: string;
  triggered_at: string;
  last_triggered_at: string;
}

export interface AlertRecord {
  id: string;
  alert_id: string;
  action: string;
  operator: string;
  comment: string;
  created_at: string;
}

export interface AlertStats {
  total: number;
  open: number;
  investigating: number;
  resolved: number;
  by_severity: Record<string, number>;
  linked_sessions: number;
  deduplicated_hit: number;
}

export interface AlertAnalysis {
  status: "ready" | "failed" | "stale";
  summary: string;
  severity_assessment: string;
  possible_causes: string[];
  suggested_actions: string[];
  recommended_tools: string[];
  knowledge_queries: string[];
  workflow: string;
  confidence: string;
  source: string;
  generated_at: string;
  error?: string;
}

export interface ToolParameter {
  name: string;
  type: "string" | "number";
  description: string;
  required: boolean;
  default?: string | number;
}

export interface ToolDefinition {
  name: string;
  display_name: string;
  description: string;
  category: string;
  allowed_roles: string[];
  parameters: ToolParameter[];
  available: boolean;
  unavailable_reason?: string;
}

export interface ToolCallLog {
  id: string;
  tool_name: string;
  operator: string;
  user_id: string;
  status: "success" | "failed" | "forbidden";
  input: Record<string, unknown>;
  output?: unknown;
  error?: string;
  duration_ms: number;
  created_at: string;
}

export interface AuditLog {
  id: string;
  category: string;
  action: string;
  status: "success" | "failed";
  actor_id?: string;
  actor_name?: string;
  actor_roles?: string[];
  target_type?: string;
  target_id?: string;
  target_name?: string;
  detail?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface AuditStats {
  total: number;
  success: number;
  failed: number;
  last_24_hours: number;
  unique_actors: number;
  by_category: Record<string, number>;
}

export async function fetchGoHealth() {
  const response = await http.get("/healthz");
  return response.data;
}

export async function fetchPythonHealth() {
  const response = await http.get("/proxy/python-ai/healthz");
  return response.data;
}

export async function login(payload: LoginPayload) {
  const response = await http.post<ApiEnvelope<LoginResponse>>("/api/v1/auth/login", payload);
  return response.data;
}

export async function fetchCurrentUser() {
  const response = await http.get<ApiEnvelope<{ user: AuthUser }>>("/api/v1/auth/me");
  return response.data;
}

export async function fetchSessions() {
  const response = await http.get<ApiEnvelope<{ sessions: ChatSession[] }>>("/api/v1/sessions");
  return response.data;
}

export async function createSession(title = "") {
  const response = await http.post<ApiEnvelope<{ session: ChatSession }>>("/api/v1/sessions", { title });
  return response.data;
}

export async function deleteSession(sessionId: string) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/api/v1/sessions/${sessionId}`);
  return response.data;
}

export async function fetchSessionMessages(sessionId: string) {
  const response = await http.get<ApiEnvelope<ChatMessagePage>>(
    `/api/v1/sessions/${sessionId}/messages`
  );
  return response.data;
}

export async function streamSessionMessage(
  sessionId: string,
  content: string,
  handlers: {
    onChunk?: (payload: { message_id: string; delta: string }) => void;
    onDone?: (payload: {
      message_id: string;
      content: string;
      route?: string;
      tool_calls?: ChatToolCall[];
      trace?: ChatTraceEvent[];
      references?: MessageReference[];
    }) => void;
  }
) {
  const token = getAccessToken();
  const response = await fetch(`${API_BASE_URL}/api/v1/sessions/${sessionId}/messages/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ content }),
  });

  if (!response.ok) {
    let message = "消息发送失败";
    try {
      const payload = await response.json();
      message = payload?.message || message;
    } catch {
      // Keep fallback message.
    }
    throw new Error(message);
  }

  if (!response.body) {
    throw new Error("流式响应不可用");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    const events = buffer.split("\n\n");
    buffer = events.pop() || "";

    for (const event of events) {
      parseSSEEvent(event, handlers);
    }
  }

  if (buffer.trim()) {
    parseSSEEvent(buffer, handlers);
  }
}

function parseSSEEvent(
  rawEvent: string,
  handlers: {
    onChunk?: (payload: { message_id: string; delta: string }) => void;
    onDone?: (payload: {
      message_id: string;
      content: string;
      route?: string;
      tool_calls?: ChatToolCall[];
      trace?: ChatTraceEvent[];
      references?: MessageReference[];
    }) => void;
  }
) {
  let eventName = "message";
  const dataLines: string[] = [];

  for (const line of rawEvent.split("\n")) {
    if (line.startsWith("event:")) {
      eventName = line.slice(6).trim();
      continue;
    }

    if (line.startsWith("data:")) {
      dataLines.push(line.slice(5).trim());
    }
  }

  if (dataLines.length === 0) {
    return;
  }

  const payload = JSON.parse(dataLines.join("\n"));
  if (eventName === "chunk") {
    handlers.onChunk?.(payload);
    return;
  }

  if (eventName === "done") {
    handlers.onDone?.(payload);
  }
}

export async function fetchKnowledgeDocuments(params?: {
  status?: string;
  category?: string;
  query?: string;
  limit?: number;
}) {
  const response = await http.get<ApiEnvelope<{ documents: KnowledgeDocument[] }>>("/api/v1/knowledge/documents", {
    params,
  });
  return response.data;
}

export async function fetchKnowledgeDocument(documentId: string) {
  const response = await http.get<ApiEnvelope<{ document: KnowledgeDocument }>>(
    `/api/v1/knowledge/documents/${documentId}`
  );
  return response.data;
}

export async function uploadKnowledgeDocument(payload: {
  title: string;
  category: string;
  content?: string;
  file?: File | null;
}) {
  const formData = new FormData();
  if (payload.title.trim()) {
    formData.append("title", payload.title.trim());
  }
  if (payload.category.trim()) {
    formData.append("category", payload.category.trim());
  }
  if (payload.content?.trim()) {
    formData.append("content", payload.content.trim());
  }
  if (payload.file) {
    formData.append("file", payload.file);
  }

  const response = await http.post<ApiEnvelope<{ document: KnowledgeDocument }>>(
    "/api/v1/knowledge/documents",
    formData,
    {
      headers: {
        "Content-Type": "multipart/form-data",
      },
    }
  );
  return response.data;
}

export async function deleteKnowledgeDocument(documentId: string) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(
    `/api/v1/knowledge/documents/${documentId}`
  );
  return response.data;
}

export async function retryKnowledgeDocument(documentId: string) {
  const response = await http.post<ApiEnvelope<{ document: KnowledgeDocument }>>(
    `/api/v1/knowledge/documents/${documentId}/reprocess`
  );
  return response.data;
}

export async function retrieveKnowledge(payload: { query: string; category?: string; limit?: number }) {
  const response = await http.post<ApiEnvelope<RetrievalReport>>("/api/v1/rag/retrieve", {
    query: payload.query,
    category: payload.category,
    limit: payload.limit,
  });
  return response.data;
}

export async function fetchAlerts(params?: { status?: string; severity?: string; service?: string; query?: string }) {
  const response = await http.get<ApiEnvelope<{ alerts: AlertItem[]; stats: AlertStats }>>("/api/v1/alerts", { params });
  return response.data;
}

export async function fetchAlertStats() {
  const response = await http.get<ApiEnvelope<{ stats: AlertStats }>>("/api/v1/alerts/stats");
  return response.data;
}

export async function fetchAlertDetail(alertId: string) {
  const response = await http.get<ApiEnvelope<{ alert: AlertItem; records: AlertRecord[] }>>(
    `/api/v1/alerts/${alertId}`
  );
  return response.data;
}

export async function updateAlertStatus(alertId: string, payload: { status: string; comment: string }) {
  const response = await http.post<ApiEnvelope<{ alert: AlertItem; records: AlertRecord[] }>>(
    `/api/v1/alerts/${alertId}/status`,
    payload
  );
  return response.data;
}

export async function linkAlertSession(alertId: string) {
  const response = await http.post<ApiEnvelope<{ alert: AlertItem; records: AlertRecord[] }>>(
    `/api/v1/alerts/${alertId}/session`
  );
  return response.data;
}

export async function analyzeAlert(alertId: string) {
  const response = await http.post<ApiEnvelope<{ alert: AlertItem; records: AlertRecord[] }>>(
    `/api/v1/alerts/${alertId}/analyze`
  );
  return response.data;
}

export async function fetchTools() {
  const response = await http.get<ApiEnvelope<{ tools: ToolDefinition[] }>>("/api/v1/tools");
  return response.data;
}

export async function callTool(toolName: string, parameters: Record<string, string | number>) {
  const response = await http.post<ApiEnvelope<{ result: unknown }>>(`/api/v1/tools/${toolName}/call`, {
    parameters,
  });
  return response.data;
}

export async function fetchToolLogs(params?: { tool_name?: string; status?: string; limit?: number }) {
  const response = await http.get<ApiEnvelope<{ logs: ToolCallLog[] }>>("/api/v1/tools/logs", {
    params,
  });
  return response.data;
}

export async function fetchAuditLogs(params?: {
  category?: string;
  action?: string;
  status?: string;
  actor?: string;
  limit?: number;
}) {
  const response = await http.get<ApiEnvelope<{ logs: AuditLog[]; categories: string[] }>>("/api/v1/audit/logs", {
    params,
  });
  return response.data;
}

export async function fetchAuditStats() {
  const response = await http.get<ApiEnvelope<{ stats: AuditStats }>>("/api/v1/audit/stats");
  return response.data;
}
