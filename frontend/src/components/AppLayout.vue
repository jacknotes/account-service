<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="brand gold-text">💰 记账本</div>
      <el-menu :default-active="route.path" router class="side-menu">
        <el-menu-item v-for="n in navRoutes" :key="n.path" :index="n.path">{{ n.title }}</el-menu-item>
      </el-menu>
      <div class="side-foot">v2.0 · Vue3 + Go</div>
    </aside>

    <div class="main">
      <header class="topbar">
        <div class="topbar-left">
          <button class="hamburger" type="button" aria-label="菜单" @click="drawerOpen = true">☰</button>
          <h1>{{ currentTitle }}</h1>
        </div>
        <div class="actions desktop-actions">
          <el-button size="small" @click="toggleTheme">{{ isLight ? '☀️ 浅色' : '🌙 深色' }}</el-button>
          <el-button size="small" @click="openPassword">修改密码</el-button>
          <el-button size="small" @click="openTOTP">TOTP</el-button>
          <span class="user-chip">{{ user?.username }} · {{ user?.role === 'admin' ? '管理员' : '用户' }}</span>
          <el-button size="small" type="danger" plain @click="doLogout">退出</el-button>
        </div>
        <div class="actions mobile-actions">
          <span class="user-chip">{{ user?.username }}</span>
          <el-button size="small" type="danger" plain @click="doLogout">退出</el-button>
        </div>
      </header>

      <main class="content">
        <RouterView />
      </main>
    </div>

    <!-- 移动端抽屉导航 -->
    <el-drawer v-model="drawerOpen" direction="ltr" size="72%" title="💰 记账本">
      <el-menu :default-active="route.path" router class="side-menu" @select="drawerOpen = false">
        <el-menu-item v-for="n in navRoutes" :key="n.path" :index="n.path">{{ n.title }}</el-menu-item>
      </el-menu>
      <div class="drawer-actions">
        <el-button @click="toggleTheme">{{ isLight ? '☀️ 浅色' : '🌙 深色' }}</el-button>
        <el-button @click="openPassword">修改密码</el-button>
        <el-button @click="openTOTP">TOTP</el-button>
      </div>
    </el-drawer>

    <!-- 修改密码 -->
    <el-dialog v-model="pwdOpen" title="修改密码" width="420px">
      <el-form label-width="90px">
        <el-form-item label="当前密码">
          <el-input v-model="pwdForm.old_password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwdForm.new_password" type="password" show-password autocomplete="new-password" placeholder="8~72 位，含大小写字母、数字、特殊字符" />
        </el-form-item>
      </el-form>
      <div class="msg-error" v-if="pwdError">{{ pwdError }}</div>
      <div class="msg-ok" v-if="pwdOk">{{ pwdOk }}</div>
      <template #footer>
        <el-button @click="pwdOpen = false">取消</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="changePassword">确认修改</el-button>
      </template>
    </el-dialog>

    <!-- TOTP 设置 -->
    <el-dialog v-model="totpOpen" :title="user?.totp_enabled ? '关闭 TOTP' : '启用 TOTP'" width="420px">
      <template v-if="!user?.totp_enabled">
        <div v-if="totpSetup">
          <p>请用身份验证器 App 扫描二维码或手动输入密钥：</p>
          <div class="qr-box"><img :src="totpQr" alt="TOTP 二维码" /></div>
          <div class="totp-secret">{{ totpSetup.secret }}</div>
          <el-form label-width="70px" style="margin-top: 12px">
            <el-form-item label="验证码">
              <el-input v-model="totpCode" placeholder="6 位验证码" inputmode="numeric" />
            </el-form-item>
          </el-form>
        </div>
        <div class="msg-error" v-if="totpError">{{ totpError }}</div>
        <div class="msg-ok" v-if="totpOk">{{ totpOk }}</div>
      </template>
      <template v-else>
        <p>关闭 TOTP 需要验证当前密码与验证码：</p>
        <el-form label-width="70px">
          <el-form-item label="当前密码">
            <el-input v-model="totpDisablePwd" type="password" show-password />
          </el-form-item>
          <el-form-item label="验证码">
            <el-input v-model="totpCode" placeholder="6 位验证码" inputmode="numeric" />
          </el-form-item>
        </el-form>
        <div class="msg-error" v-if="totpError">{{ totpError }}</div>
        <div class="msg-ok" v-if="totpOk">{{ totpOk }}</div>
      </template>
      <template #footer>
        <el-button @click="totpOpen = false">取消</el-button>
        <el-button v-if="!user?.totp_enabled" type="primary" :disabled="!totpCode" @click="enableTOTP">启用</el-button>
        <el-button v-else type="danger" plain :disabled="!totpCode || !totpDisablePwd" @click="disableTOTP">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiFetch } from '../api/http'
import { getUser, setUser, clearSession, getRefreshToken } from '../api/auth'

const route = useRoute()
const router = useRouter()

const nav = [
  { path: '/records', title: '记账' },
  { path: '/summary', title: '汇总' },
  { path: '/report', title: '报表' },
  { path: '/categories', title: '分类' },
  { path: '/users', title: '用户管理', admin: true },
  { path: '/logs', title: '操作日志', admin: true },
]
const navRoutes = computed(() => nav.filter((n) => !n.admin || getUser()?.role === 'admin'))
const currentTitle = computed(() => (route.meta && route.meta.title) || '')

const user = ref(getUser())
const drawerOpen = ref(false)

// 主题（默认深色；同时驱动 Element Plus 的 html.dark）
const isLight = ref(localStorage.getItem('theme') === 'light')
function applyTheme() {
  document.documentElement.classList.toggle('dark', !isLight.value)
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
