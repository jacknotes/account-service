// token / 用户信息本地持久化
// 注意：access token 存 localStorage 存在 XSS 风险；已通过 refresh token 服务端
// 轮换与撤销缓解（即使 token 泄露，15 分钟即失效，且登出/改密可立即撤销）。
const TOKEN_KEY = 'account_token'
const REFRESH_KEY = 'account_refresh'
const USER_KEY = 'account_user'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function getRefreshToken() {
  return localStorage.getItem(REFRESH_KEY)
}

export function setTokens(token, refreshToken) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(REFRESH_KEY, refreshToken)
}

export function setUser(user) {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function getUser() {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY))
  } catch {
    return null
  }
}

export function getRole() {
  const u = getUser()
  return (u && u.role) || 'user'
}

export function isAdmin() {
  return getRole() === 'admin'
}

export function isLoggedIn() {
  return !!getToken()
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_KEY)
  localStorage.removeItem(USER_KEY)
}
