<template>
  <div class="card">
    <div class="filter-bar">
      <div class="form-row">
        <label>用户 ID</label>
        <input v-model="filters.user_id" type="number" placeholder="全部" @keyup.enter="reload(1)" />
      </div>
      <div class="form-row">
        <label>操作类型</label>
        <select v-model="filters.action" @change="reload(1)">
          <option value="">全部</option>
          <option v-for="(name, key) in actionNames" :key="key" :value="key">{{ name }}</option>
        </select>
      </div>
      <button class="btn" type="button" @click="reload(1)">查询</button>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>ID</th>
            <th>用户</th>
            <th>操作</th>
            <th>详情</th>
            <th>IP</th>
            <th>时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!list.length">
            <td class="empty" colspan="6">{{ loading ? '加载中...' : '暂无日志' }}</td>
          </tr>
          <tr v-for="l in list" :key="l.id">
            <td class="num">{{ l.id }}</td>
            <td>{{ l.username }} ({{ l.user_id }})</td>
            <td>{{ l.action_name || l.action }}</td>
            <td>{{ l.detail || '-' }}</td>
            <td class="num">{{ l.ip || '-' }}</td>
            <td class="num">{{ l.created_at }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <Pagination :page="page" :page-size="pageSize" :total="total" @change="reload" />
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Pagination from '../components/Pagination.vue'
import { api } from '../api/http'

const actionNames = {
  login: '登录',
  logout: '退出',
  refresh: '刷新会话',
  create_record: '创建记账',
  update_record: '更新记账',
  delete_record: '删除记账',
  add_user: '添加用户',
  update_user: '更新用户',
  delete_user: '删除用户',
  change_password: '修改密码',
  totp_enable: '启用TOTP',
  totp_disable: '关闭TOTP',
}

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const filters = reactive({ user_id: '', action: '' })

async function reload(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params = new URLSearchParams()
    params.set('page', page.value)
    params.set('page_size', pageSize.value)
    if (filters.user_id) params.set('user_id', filters.user_id)
    if (filters.action) params.set('action', filters.action)
    const data = await api('/api/auth/operation-logs?' + params.toString())
    list.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    alert(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => reload(1))
</script>
