<template>
  <div class="card">
    <div class="filter-bar" style="align-items: center">
      <el-input-number
        v-model="userId"
        :min="1"
        placeholder="用户 ID（全部）"
        controls-position="right"
        style="width: 160px"
        @keyup.enter="reload(1)"
      />
      <el-select v-model="action" placeholder="操作类型（全部）" clearable style="width: 170px" @change="reload(1)">
        <el-option v-for="(name, key) in actionNames" :key="key" :label="name" :value="key" />
      </el-select>
      <el-button type="primary" @click="reload(1)">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" empty-text="暂无日志">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="用户" width="150">
        <template #default="{ row }">{{ row.username }}（{{ row.user_id }}）</template>
      </el-table-column>
      <el-table-column label="操作" width="110">
        <template #default="{ row }">{{ row.action_name || row.action }}</template>
      </el-table-column>
      <el-table-column prop="detail" label="详情" show-overflow-tooltip />
      <el-table-column label="IP" width="130">
        <template #default="{ row }">{{ row.ip || '-' }}</template>
      </el-table-column>
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ row.created_at }}</template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      background
      layout="total, prev, pager, next"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="reload"
    />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/http'

const actionNames = {
  login: '登录',
  logout: '退出',
  refresh: '刷新会话',
  create_record: '创建记账',
  update_record: '更新记账',
  delete_record: '删除记账',
  create_category: '新增分类',
  delete_category: '删除分类',
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
const userId = ref(null)
const action = ref('')

async function reload(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params = new URLSearchParams()
    params.set('page', page.value)
    params.set('page_size', pageSize.value)
    if (userId.value) params.set('user_id', userId.value)
    if (action.value) params.set('action', action.value)
    const data = await api('/api/auth/operation-logs?' + params.toString())
    list.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => reload(1))
</script>
