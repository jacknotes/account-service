<template>
  <div>
    <div class="card">
      <div class="filter-bar">
        <h3 style="margin: 0; flex: 1">用户管理</h3>
        <button class="btn btn-primary" type="button" @click="openAdd">+ 添加用户</button>
      </div>

      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>ID</th>
              <th>用户名</th>
              <th>角色</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!users.length">
              <td class="empty" colspan="5">{{ loading ? '加载中...' : '暂无用户' }}</td>
            </tr>
            <tr v-for="u in users" :key="u.id">
              <td class="num">{{ u.id }}</td>
              <td>{{ u.username }}</td>
              <td><span class="badge" :class="{ admin: u.role === 'admin' }">{{ u.role === 'admin' ? '管理员' : '用户' }}</span></td>
              <td class="num">{{ (u.created_at || '').slice(0, 10) }}</td>
              <td>
                <div class="actions-inline">
                  <button class="btn btn-sm" type="button" @click="openEdit(u)">编辑</button>
                  <button class="btn btn-sm" type="button" @click="openChangePwd(u)">改密</button>
                  <button class="btn btn-sm btn-danger" type="button" @click="askDelete(u)">删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 添加 -->
    <Modal v-model="addOpen" title="添加用户">
      <div class="form-row">
        <label>用户名（2~32 字符）</label>
        <input v-model="addForm.username" />
      </div>
      <div class="form-row">
        <label>密码（8~72 位，含大小写字母、数字、特殊字符）</label>
        <input v-model="addForm.password" type="password" />
      </div>
      <div class="form-row">
        <label>角色</label>
        <select v-model="addForm.role">
          <option value="user">用户</option>
          <option value="admin">管理员</option>
        </select>
      </div>
      <div class="msg-error">{{ error }}</div>
      <template #footer>
        <button class="btn" type="button" @click="addOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="saving" @click="addUser">{{ saving ? '保存中...' : '添加' }}</button>
      </template>
    </Modal>

    <!-- 编辑 -->
    <Modal v-model="editOpen" title="编辑用户">
      <div class="form-row">
        <label>用户名</label>
        <input v-model="editForm.username" />
      </div>
      <div class="form-row">
        <label>角色</label>
        <select v-model="editForm.role">
          <option value="user">用户</option>
          <option value="admin">管理员</option>
        </select>
      </div>
      <div class="msg-error">{{ error }}</div>
      <template #footer>
        <button class="btn" type="button" @click="editOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="saving" @click="updateUser">保存</button>
      </template>
    </Modal>

    <!-- 改密 -->
    <Modal v-model="pwdOpen" :title="'修改用户密码：' + (pwdTarget?.username || '')">
      <div class="form-row">
        <label>新密码（8~72 位，含大小写字母、数字、特殊字符）</label>
        <input v-model="pwdForm.password" type="password" />
      </div>
      <div class="msg-error">{{ error }}</div>
      <template #footer>
        <button class="btn" type="button" @click="pwdOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="saving" @click="changePwd">确认</button>
      </template>
    </Modal>

    <!-- 删除确认 -->
    <Modal v-model="delOpen" title="删除用户">
      <p>删除用户将级联删除其全部记账记录，此操作不可撤销！</p>
      <template #footer>
        <button class="btn" type="button" @click="delOpen = false">取消</button>
        <button class="btn btn-danger" type="button" :disabled="deleting" @click="doDelete">删除</button>
      </template>
    </Modal>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Modal from '../components/Modal.vue'
import { api } from '../api/http'
import { getUser } from '../api/auth'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const error = ref('')

const addOpen = ref(false)
const addForm = reactive({ username: '', password: '', role: 'user' })
const editOpen = ref(false)
const editForm = reactive({ id: null, username: '', role: 'user' })
const pwdOpen = ref(false)
const pwdTarget = ref(null)
const pwdForm = reactive({ password: '' })
const delOpen = ref(false)
const delTarget = ref(null)

const me = getUser()

async function load() {
  loading.value = true
  try {
    const data = await api('/api/auth/users')
    users.value = data.data || []
  } catch (e) {
    error.value = e.message
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
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

function askDelete(u) {
  delTarget.value = u
  delOpen.value = true
}

async function doDelete() {
  deleting.value = true
  try {
    await api('/api/auth/users/' + delTarget.value.id, { method: 'DELETE' })
    delOpen.value = false
    await load()
  } catch (err) {
    alert(err.message)
  } finally {
    deleting.value = false
  }
}

onMounted(load)
</script>
