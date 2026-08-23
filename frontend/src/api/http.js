// fetch 封装：自动携带 Authorization；401 时用 refresh token 换新并重试一次；
// refresh 也失败则清空会话并跳转登录页。
import { getToken, getRefreshToken, setTokens, setUser, clearSession } from './auth'

let refreshPromise = null

async function refreshToken() {
  if (refreshPromise) return refreshPromise
  refreshPromise = (async () => {
    const rt = getRefreshToken()
    if (!rt) throw new Error('no refresh token')
    const res = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: rt }),
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '刷新会话失败')
    setTokens(data.token, data.refresh_token)
    if (data.user) setUser(data.user)
    return data.token
  })().finally(() => {
    refreshPromise = null
  })
  return refreshPromise
}

function redirectLogin() {
  if (typeof window !== 'undefined' && !window.location.hash.includes('login')) {
    window.location.hash = '#/login'
  }
}

// 低层 fetch：返回 Response（不解析 body），失败时已处理跳转。
export async function apiFetch(url, options = {}) {
  const opts = { ...options, headers: { 'Content-Type': 'application/json', ...(options.headers || {}) } }
  const token = getToken()
  if (token) opts.headers['Authorization'] = 'Bearer ' + token

  const res = await fetch(url, opts)
  if (res.status === 401 && !opts._retried && getRefreshToken()) {
    try {
      await refreshToken()
      opts._retried = true
      opts.headers['Authorization'] = 'Bearer ' + getToken()
      const retry = await fetch(url, opts)
      if (retry.status === 401) {
        clearSession()
        redirectLogin()
      }
      return retry
    } catch {
      clearSession()
      redirectLogin()
      throw new Error('登录已过期，请重新登录')
    }
  }
  if (res.status === 401) {
    clearSession()
    redirectLogin()
  }
  return res
}

// 高层 API：解析 JSON，非 2xx 抛错（error 字段）。
export async function api(url, options = {}) {
  const res = await apiFetch(url, options)
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `请求失败 (${res.status})`)
  }
  return data
}
