<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="brand">💰 记账本</div>
      <nav>
        <RouterLink v-for="r in navRoutes" :key="r.path" :to="r.path">{{ r.title }}</RouterLink>
      </nav>
      <div class="side-foot">v1.0 · Vue3 + Go</div>
    </aside>

    <div class="main">
      <header class="topbar">
        <h1>{{ currentTitle }}</h1>
        <div class="actions">
          <button class="btn" type="button" @click="toggleTheme">
            {{ isLight ? '☀️ 浅色' : '🌙 深色' }}
          </button>
          <button class="btn" type="button" @click="openPassword">修改密码</button>
          <button class="btn" type="button" @click="openTOTP">TOTP</button>
          <span class="user-chip">{{ user?.username }} · {{ user?.role === 'admin' ? '管理员' : '用户' }}</span>
          <button class="btn btn-danger" type="button" @click="doLogout">退出</button>
        </div>
      </header>

      <main class="content">
        <RouterView />
      </main>
    </div>

    <!-- 修改密码 -->
    <Modal v-model="pwdOpen" title="修改密码">
      <div class="form-row">
        <label>当前密码</label>
        <input v-model="pwdForm.old_password" type="password" autocomplete="current-password" />
      </div>
      <div class="form-row">
        <label>新密码（8~72 位，含大小写字母、数字、特殊字符）</label>
        <input v-model="pwdForm.new_password" type="password" autocomplete="new-password" />
      </div>
      <div class="msg-error">{{ pwdError }}</div>
      <div class="msg-ok" v-if="pwdOk">{{ pwdOk }}</div>
      <template #footer>
        <button class="btn" type="button" @click="pwdOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="pwdLoading" @click="changePassword">确认修改</button>
      </template>
    </Modal>

    <!-- TOTP 设置 -->
    <Modal v-model="totpOpen" :title="user?.totp_enabled ? '关闭 TOTP' : '启用 TOTP'">
      <template v-if="!user?.totp_enabled">
        <div v-if="totpSetup">
          <p>请用身份验证器 App 扫描二维码或手动输入密钥：</p>
          <div class="qr-box"><img :src="totpQr" alt="TOTP 二维码" /></div>
          <div class="totp-secret">{{ totpSetup.secret }}</div>
          <div class="form-row" style="margin-top: 12px">
            <label>验证码</label>
            <input v-model="totpCode" placeholder="6 位验证码" inputmode="numeric" />
          </div>
        </div>
        <div class="msg-error">{{ totpError }}</div>
        <div class="msg-ok" v-if="totpOk">{{ totpOk }}</div>
      </template>
      <template v-else>
        <p>关闭 TOTP 需要验证当前密码与验证码：</p>
        <div class="form-row">
          <label>当前密码</label>
          <input v-model="totpDisablePwd" type="password" />
        </div>
        <div class="form-row">
          <label>验证码</label>
          <input v-model="totpCode" placeholder="6 位验证码" inputmode="numeric" />
        </div>
        <div class="msg-error">{{ totpError }}</div>
        <div class="msg-ok" v-if="totpOk">{{ totpOk }}</div>
      </template>
      <template #footer>
        <button class="btn" type="button" @click="totpOpen = false">取消</button>
        <button v-if="!user?.totp_enabled" class="btn btn-primary" type="button" :disabled="!totpCode" @click="enableTOTP">启用</button>
        <button v-else class="btn btn-danger" type="button" :disabled="!totpCode || !totpDisablePwd" @click="disableTOTP">关闭</button>
      </template>
    </Modal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Modal from './Modal.vue'
import { api, apiFetch } from '../api/http'
import { getUser, setUser, clearSession, getRefreshToken } from '../api/auth'

const route = useRoute()
const router = useRouter()

const nav = [
  { path: '/records', title: '记账' },
  { path: '/summary', title: '汇总' },
  { path: '/report', title: '报表' },
  { path: '/users', title: '用户管理', admin: true },
  { path: '/logs', title: '操作日志', admin: true },
]
const navRoutes = computed(() => nav.filter((n) => !n.admin || getUser()?.role === 'admin'))
const currentTitle = computed(() => (route.meta && route.meta.title) || '')

const user = ref(getUser())

// 主题
const isLight = ref(localStorage.getItem('theme') === 'light')
function applyTheme() {
  document.body.classList.toggle('theme-light', isLight.value)
}
function toggleTheme() {
  isLight.value = !isLight.value
  localStorage.setItem('theme', isLight.value ? 'light' : 'dark')
  applyTheme()
}
onMounted(applyTheme)

// 刷新用户信息
async function refreshUser() {
  try {
    const data = await api('/api/auth/me')
    setUser({ id: data.id, username: data.username, role: data.role, totp_enabled: data.totp_enabled })
    user.value = getUser()
  } catch {
    /* 未登录场景忽略 */
  }
}
onMounted(refreshUser)

// 退出登录
async function doLogout() {
  try {
    await apiFetch('/api/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: getRefreshToken() }),
    })
  } catch {
    /* 忽略登出错误，本地仍清理 */
  }
  clearSession()
  router.push({ name: 'login' })
}

// 修改密码
const pwdOpen = ref(false)
const pwdLoading = ref(false)
const pwdError = ref('')
const pwdOk = ref('')
const pwdForm = reactive({ old_password: '', new_password: '' })
function openPassword() {
  pwdError.value = ''
  pwdOk.value = ''
  pwdForm.old_password = ''
  pwdForm.new_password = ''
  pwdOpen.value = true
}
async function changePassword() {
  pwdError.value = ''
  pwdOk.value = ''
  const err = validatePassword(pwdForm.new_password)
  if (err) {
    pwdError.value = err
    return
  }
  pwdLoading.value = true
  try {
    await api('/api/auth/change-password', {
      method: 'POST',
      body: JSON.stringify(pwdForm),
    })
    pwdOk.value = '密码已修改，请重新登录'
    setTimeout(() => {
      clearSession()
      router.push({ name: 'login' })
    }, 1200)
  } catch (e) {
    pwdError.value = e.message
  } finally {
    pwdLoading.value = false
  }
}

// TOTP
const totpOpen = ref(false)
const totpSetup = ref(null)
const totpQr = ref('')
const totpCode = ref('')
const totpDisablePwd = ref('')
const totpError = ref('')
const totpOk = ref('')

async function openTOTP() {
  totpError.value = ''
  totpOk.value = ''
  totpCode.value = ''
  totpDisablePwd.value = ''
  totpOpen.value = true
  if (user.value && !user.value.totp_enabled) {
    await loadTOTPSetup()
  }
}

async function loadTOTPSetup() {
  try {
    const data = await api('/api/auth/totp/setup')
    totpSetup.value = data
    const QRCode = (await import('qrcode')).default
    totpQr.value = await QRCode.toDataURL(data.url)
  } catch (e) {
    totpError.value = e.message
  }
}

async function enableTOTP() {
  totpError.value = ''
  try {
    await api('/api/auth/totp/enable', {
      method: 'POST',
      body: JSON.stringify({ secret: totpSetup.value.secret, code: totpCode.value }),
    })
    totpOk.value = 'TOTP 已启用'
    user.value = { ...user.value, totp_enabled: true }
    setUser(user.value)
    setTimeout(() => (totpOpen.value = false), 1200)
  } catch (e) {
    totpError.value = e.message
  }
}

async function disableTOTP() {
  totpError.value = ''
  try {
    await api('/api/auth/totp/disable', {
      method: 'POST',
      body: JSON.stringify({ password: totpDisablePwd.value, code: totpCode.value }),
    })
    totpOk.value = 'TOTP 已关闭'
    user.value = { ...user.value, totp_enabled: false }
    setUser(user.value)
    setTimeout(() => (totpOpen.value = false), 1200)
  } catch (e) {
    totpError.value = e.message
  }
}

// 密码强度校验（与后端一致）
function validatePassword(pwd) {
  const bytes = new TextEncoder().encode(pwd).length
  if (bytes < 8) return '密码长度不能少于 8 位'
  if (bytes > 72) return '密码过长，不能超过 72 字节'
  if (!/[A-Z]/.test(pwd)) return '密码须包含大写字母'
  if (!/[a-z]/.test(pwd)) return '密码须包含小写字母'
  if (!/\d/.test(pwd)) return '密码须包含数字'
  if (!/[^A-Za-z0-9]/.test(pwd)) return '密码须包含特殊字符'
  return ''
}

// 主题初始化 + 监听（router 守卫可能先跳转）
watch(isLight, applyTheme)
</script>
