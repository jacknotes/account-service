<template>
  <div class="card">
    <div class="page-head">
      <h3>用户管理</h3>
      <el-button type="primary" @click="openAdd">＋ 添加用户</el-button>
    </div>

    <el-table :data="users" v-loading="loading" empty-text="暂无用户">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'warning' : 'info'" effect="dark" size="small">
            {{ row.role === 'admin' ? '管理员' : '用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="130">
        <template #default="{ row }">{{ (row.created_at || '').slice(0, 10) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" text @click="openEdit(row)">编辑</el-button>
          <el-button size="small" text @click="openChangePwd(row)">改密</el-button>
          <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <!-- 添加用户 -->
  <el-dialog v-model="addOpen" title="添加用户" width="420px">
    <el-form label-width="70px">
      <el-form-item label="用户名">
        <el-input v-model="addForm.username" placeholder="2~32 字符" />
      </el-form-item>
      <el-form-item label="密码">
        <el-input v-model="addForm.password" type="password" show-password placeholder="8~72 位，含大小写字母、数字、特殊字符" />
      </el-form-item>
      <el-form-item label="角色">
        <el-radio-group v-model="addForm.role">
          <el-radio value="user">用户</el-radio>
          <el-radio value="admin">管理员</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="addOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="addUser">添加</el-button>
    </template>
  </el-dialog>

  <!-- 编辑用户 -->
  <el-dialog v-model="editOpen" title="编辑用户" width="420px">
    <el-form label-width="70px">
      <el-form-item label="用户名">
        <el-input v-model="editForm.username" />
      </el-form-item>
      <el-form-item label="角色">
        <el-radio-group v-model="editForm.role">
          <el-radio value="user">用户</el-radio>
          <el-radio value="admin">管理员</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="editOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="updateUser">保存</el-button>
    </template>
  </el-dialog>

  <!-- 修改用户密码 -->
  <el-dialog v-model="pwdOpen" :title="'修改用户密码：' + (pwdTarget?.username || '')" width="420px">
    <el-form label-width="70px">
      <el-form-item label="新密码">
        <el-input v-model="pwdForm.password" type="password" show-password placeholder="8~72 位，含大小写字母、数字、特殊字符" />
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="pwdOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="changePwd">确认</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api/http'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')

const addOpen = ref(false)
const addForm = reactive({ username: '', password: '', role: 'user' })
const editOpen = ref(false)
const editForm = reactive({ id: null, username: '', role: 'user' })
const pwdOpen = ref(false)
const pwdTarget = ref(null)
const pwdForm = reactive({ password: '' })

async function load() {
  loading.value = true
  try {
    const data = await api('/api/auth/users')
    users.value = data.data || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function validatePwd(pwd) {
  const bytes = new TextEncoder().encode(pwd).length
  if (bytes < 8 || bytes > 72) return '密码长度需 8~72 字节'
  if (!/[A-Z]/.test(pwd) || !/[a-z]/.test(pwd) || !/\d/.test(pwd) || !/[^A-Za-z0-9]/.test(pwd)) {
    return '密码需包含大小写字母、数字、特殊字符'
  }
  return ''
}

function openAdd() {
  error.value = ''
  addForm.username = ''
  addForm.password = ''
  addForm.role = 'user'
  addOpen.value = true
}

async function addUser() {
  error.value = ''
  const e = validatePwd(addForm.password)
  if (e) {
    error.value = e
    return
  }
  saving.value = true
  try {
    await api('/api/auth/users', { method: 'POST', body: JSON.stringify(addForm) })
    addOpen.value = false
    ElMessage.success('用户已添加')
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

function openEdit(u) {
  error.value = ''
  editForm.id = u.id
  editForm.username = u.username
  editForm.role = u.role
  editOpen.value = true
}

async function updateUser() {
  error.value = ''
  saving.value = true
  try {
    await api('/api/auth/users/' + editForm.id, {
      method: 'PUT',
      body: JSON.stringify({ username: editForm.username, role: editForm.role }),
    })
    editOpen.value = false
    ElMessage.success('已更新')
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

function openChangePwd(u) {
  error.value = ''
  pwdTarget.value = u
  pwdForm.password = ''
  pwdOpen.value = true
}

async function changePwd() {
  error.value = ''
  const e = validatePwd(pwdForm.password)
  if (e) {
    error.value = e
    return
  }
  saving.value = true
  try {
    await api('/api/auth/users/' + pwdTarget.value.id + '/change-password', {
      method: 'POST',
      body: JSON.stringify({ password: pwdForm.password }),
    })
    pwdOpen.value = false
    ElMessage.success('密码已修改')
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

async function askDelete(u) {
  try {
    await ElMessageBox.confirm('删除用户将级联删除其全部记账记录，此操作不可撤销！', '删除用户', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await api('/api/auth/users/' + u.id, { method: 'DELETE' })
    ElMessage.success('已删除')
    await load()
  } catch (err) {
    ElMessage.error(err.message)
  }
}

onMounted(load)
</script>
