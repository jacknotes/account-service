<template>
  <div class="auth-page">
    <el-card class="auth-card" shadow="always">
      <h2 class="gold-text" style="margin: 0 0 20px; text-align: center">💰 记账本</h2>

      <el-form v-if="!showRegister" label-position="top" @submit.prevent="login">
        <el-form-item label="用户名">
          <el-input v-model.trim="form.username" autocomplete="username" placeholder="用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="current-password" placeholder="密码" @keyup.enter="login" />
        </el-form-item>
        <el-form-item v-if="needsTOTP" label="TOTP 验证码">
          <el-input v-model="form.totp_code" placeholder="6 位验证码" inputmode="numeric" autocomplete="one-time-code" @keyup.enter="login" />
        </el-form-item>
        <el-button type="primary" native-type="submit" style="width: 100%" size="large" :loading="loading">
          {{ loading ? '登录中...' : '登录' }}
        </el-button>
        <div class="msg-error" v-if="error">{{ error }}</div>
      </el-form>

      <el-form v-else label-position="top" @submit.prevent="register">
        <el-form-item label="用户名（2~32 字符）">
          <el-input v-model.trim="regForm.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码（8~72 位，含大小写字母、数字、特殊字符）">
          <el-input v-model="regForm.password" type="password" show-password autocomplete="new-password" @keyup.enter="register" />
        </el-form-item>
        <el-button type="primary" native-type="submit" style="width: 100%" size="large" :loading="loading">
          {{ loading ? '注册中...' : '注册并登录' }}
        </el-button>
        <div class="msg-error" v-if="error">{{ error }}</div>
      </el-form>

      <div class="auth-switch" v-if="showRegister || allowRegister">
        <span v-if="!showRegister">尚无账号？</span>
        <a @click="showRegister = !showRegister">{{ showRegister ? '返回登录' : '注册首个管理员账号' }}</a>
      </div>
    </el-card>
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
