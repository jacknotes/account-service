const API = '/api';
const TOKEN_KEY = 'account_token';

function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token);
}

function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

function isLoggedIn() {
  return !!getToken();
}

function authHeaders() {
  const token = getToken();
  const h = { 'Content-Type': 'application/json' };
  if (token) h['Authorization'] = 'Bearer ' + token;
  return h;
}

async function fetchAuth(url, options = {}) {
  const opt = { ...options, headers: { ...authHeaders(), ...options.headers } };
  const res = await fetch(url, opt);
  if (res.status === 401) {
    clearToken();
    if (typeof window !== 'undefined' && !window.location.pathname.includes('login')) {
      window.location.href = '/app/login.html';
    }
    throw new Error('未登录');
  }
  return res;
}

function logout() {
  clearToken();
  window.location.href = '/app/login.html';
}
