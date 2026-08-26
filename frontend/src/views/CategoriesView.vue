<template>
  <div class="card">
    <div class="page-head">
      <h3>分类管理</h3>
      <el-button type="primary" @click="openAdd">＋ 新增分类</el-button>
    </div>

    <el-tabs v-model="activeTab">
      <el-tab-pane label="支出分类" name="expense">
        <el-table :data="catsOf('expense')" v-loading="loading" empty-text="暂无分类">
          <el-table-column prop="name" label="名称" />
          <el-table-column label="创建时间" width="140">
            <template #default="{ row }">{{ (row.created_at || '').slice(0, 10) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="收入分类" name="income">
        <el-table :data="catsOf('income')" v-loading="loading" empty-text="暂无分类">
          <el-table-column prop="name" label="名称" />
          <el-table-column label="创建时间" width="140">
            <template #default="{ row }">{{ (row.created_at || '').slice(0, 10) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>

  <el-dialog v-model="addOpen" title="新增分类" width="400px">
    <el-form label-width="64px">
      <el-form-item label="名称">
        <el-input v-model="addForm.name" maxlength="64" show-word-limit placeholder="如：餐饮" @keyup.enter="addCategory" />
      </el-form-item>
      <el-form-item label="类型">
        <el-radio-group v-model="addForm.type">
          <el-radio value="expense">支出</el-radio>
          <el-radio value="income">收入</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="addOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="addCategory">添加</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api/http'

const activeTab = ref('expense')
const categories = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const addOpen = ref(false)
const addForm = reactive({ name: '', type: 'expense' })

function catsOf(type) {
  return categories.value.filter((c) => c.type === type)
}

async function load() {
  loading.value = true
  try {
    const data = await api('/api/categories')
    categories.value = data.data || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openAdd() {
  error.value = ''
  addForm.name = ''
  addForm.type = activeTab.value
  addOpen.value = true
}

async function addCategory() {
  error.value = ''
  const name = addForm.name.trim()
  if (!name) {
    error.value = '请填写分类名称'
    return
  }
  saving.value = true
  try {
    await api('/api/categories', { method: 'POST', body: JSON.stringify({ name, type: addForm.type }) })
    addOpen.value = false
    ElMessage.success('分类已添加')
    activeTab.value = addForm.type
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function askDelete(row) {
  try {
    await ElMessageBox.confirm(
      `删除分类「${row.name}」后，历史记录中的该分类文字不受影响。确定删除？`,
      '删除分类',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  try {
    await api('/api/categories/' + row.id, { method: 'DELETE' })
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>
