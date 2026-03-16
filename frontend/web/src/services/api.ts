import { http } from "@/services/http";

export async function fetchGoHealth() {
  const response = await http.get("/healthz");
  return response.data;
}

export async function fetchPythonHealth() {
  const response = await http.get("/proxy/python-ai/healthz");
  return response.data;
}
