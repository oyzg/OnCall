const TOKEN_STORAGE_KEY = "oncall_token";

let accessToken = localStorage.getItem(TOKEN_STORAGE_KEY) || "";

export function getAccessToken() {
  return accessToken;
}

export function setAccessToken(token: string) {
  accessToken = token;
  localStorage.setItem(TOKEN_STORAGE_KEY, token);
}

export function clearAccessToken() {
  accessToken = "";
  localStorage.removeItem(TOKEN_STORAGE_KEY);
}
