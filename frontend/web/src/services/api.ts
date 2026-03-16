import { http } from "@/services/http";

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
