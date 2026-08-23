<template>
  <div class="auth-page">
    <div class="auth-card">
      <h2>💰 记账本</h2>

      <form v-if="!showRegister" @submit.prevent="login">
        <div class="form-row">
          <label>用户名</label>
          <input v-model.trim="form.username" autocomplete="username" />
        </div>
        <div class="form-row">
          <label>密码</label>
          <input v-model="form.password" type="password" autocomplete="current-password" />
        </div>
        <div v-if="needsTOTP" class="form-row">
          <label>TOTP 验证码</label>
          <input v-model="form.totp_code" inputmode="numeric" placeholder="6 位验证码" autocomplete="one-time-code" />
        </div>
        <button class="btn btn-primary" style="width: 100%" type="submit" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
        <div class="msg-error">{{ error }}</div>
      </form>

      <form v-else @submit.prevent="register">
        <div class="form-row">
          <label>用户名（2~32 字符）</label>
          <input v-model.trim="regForm.username" autocomplete="username" />
        </div>
        <div class="form-row">
          <label>密码（8~72 位，含大小写字母、数字、特殊字符）</label>
          <input v-model="regForm.password" type="password" autocomplete="new-password" />
        </div>
        <button class="btn btn-primary" style="width: 100%" type="submit" :disabled="loading">
          {{ loading ? '注册中...' : '注册并登录' }}
        </button>
        <div class="msg-error">{{ error }}</div>
      </form>

      <div class="auth-switch" v-if="showRegister || allowRegister">
        <span v-if="!showRegister">尚无账号？</span>
        <a @click="showRegister = !showRegister">{{ showRegister ? '返回登录' : '注册首个管理员账号' }}</a>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/http'
import { setTokens, setUser } from '../api/auth'

const router = useRouter()
const showRegister = ref(false)
const allowRegister = ref(false)
const loading = ref(false)
const error = ref('')
const needsTOTP = ref(false)
const form = reactive({ username: '', password: '', totp_code: '' })
const regForm = reactive({ username: '', password: '' })

async function afterAuth(data) {
  setTokens(data.token, data.refresh_token)
  if (data.user) setUser(data.user)
  router.push({ path: '/' })
}

async function login() {
  error.value = ''
  if (!form.username || !form.password) {
    error.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  try {
    const data = await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username: form.username, password: form.password, totp_code: form.totp_code || undefined }),
    })
    if (data.needs_totp) {
      needsTOTP.value = true
      error.value = '请输入 TOTP 验证码'
      return
    }
    await afterAuth(data)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function register() {
  error.value = ''
  if (!regForm.username || !regForm.password) {
    error.value = '请输入用户名和密码'
    return
  }
  const bytes = new TextEncoder().encode(regForm.password).length
  if (bytes < 8 || bytes > 72 || !/[A-Z]/.test(regForm.password) || !/[a-z]/.test(regForm.password) || !/\d/.test(regForm.password) || !/[^A-Za-z0-9]/.test(regForm.password)) {
    error.value = '密码需 8~72 位且包含大小写字母、数字、特殊字符'
    return
  }
  loading.value = true
  try {
    const data = await api('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(regForm),
    })
    await afterAuth(data)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const data = await api('/api/auth/register/status')
    allowRegister.value = !!data.allow_register
  } catch {
    allowRegister.value = false
  }
})
</script>
