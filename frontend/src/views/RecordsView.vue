<template>
  <div>
    <div class="card">
      <div class="filter-bar">
        <div class="form-row">
          <label>开始日期</label>
          <input v-model="filters.start_date" type="date" @change="reload(1)" />
        </div>
        <div class="form-row">
          <label>结束日期</label>
          <input v-model="filters.end_date" type="date" @change="reload(1)" />
        </div>
        <div class="form-row" style="flex: 1; min-width: 160px">
          <label>关键字（描述/分类）</label>
          <input v-model="filters.keyword" placeholder="搜索..." @keyup.enter="reload(1)" />
        </div>
        <button class="btn" type="button" @click="reload(1)">查询</button>
        <button class="btn btn-primary" type="button" @click="openAdd">+ 记一笔</button>
      </div>

      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th class="sortable" @click="toggleSort('date')">日期 {{ sortArrow('date') }}</th>
              <th class="sortable" @click="toggleSort('amount')">金额 {{ sortArrow('amount') }}</th>
              <th class="sortable" @click="toggleSort('category')">分类 {{ sortArrow('category') }}</th>
              <th>描述</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!list.length">
              <td class="empty" colspan="5">{{ loading ? '加载中...' : '暂无记录' }}</td>
            </tr>
            <tr v-for="r in list" :key="r.id">
              <td class="num">{{ r.date }}</td>
              <td class="num" :class="r.amount_cents >= 0 ? 'income' : 'expense'">{{ formatCents(r.amount_cents) }}</td>
              <td>{{ r.category || '-' }}</td>
              <td>{{ r.description || '-' }}</td>
              <td>
                <div class="actions-inline">
                  <button class="btn btn-sm" type="button" @click="openEdit(r)">编辑</button>
                  <button class="btn btn-sm btn-danger" type="button" @click="askDelete(r)">删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <Pagination :page="page" :page-size="pageSize" :total="total" @change="reload" />
    </div>

    <!-- 新增 / 编辑 -->
    <Modal v-model="editOpen" :title="editingId ? '编辑记录' : '记一笔'">
      <div class="form-row">
        <label>日期</label>
        <input v-model="form.date" type="date" required />
      </div>
      <div class="form-row">
        <label>金额（元，正数收入 / 负数支出）</label>
        <input v-model="form.amountYuan" type="number" step="0.01" required />
      </div>
      <div class="form-row">
        <label>分类</label>
        <input v-model="form.category" maxlength="64" placeholder="如：餐饮" />
      </div>
      <div class="form-row">
        <label>描述</label>
        <input v-model="form.description" maxlength="255" placeholder="备注" />
      </div>
      <div class="msg-error">{{ error }}</div>
      <template #footer>
        <button class="btn" type="button" @click="editOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="saving" @click="save">{{ saving ? '保存中...' : '保存' }}</button>
      </template>
    </Modal>

    <!-- 删除确认 -->
    <Modal v-model="delOpen" title="删除记录">
      <p>确定删除该记录吗？此操作不可撤销。</p>
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
import Pagination from '../components/Pagination.vue'
import { api } from '../api/http'
import { formatCents, centsToYuan, yuanToCents, today } from '../utils/format'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const filters = reactive({ start_date: '', end_date: '', keyword: '' })
const sortField = ref('date')
const sortDir = ref('desc')

const editOpen = ref(false)
const editingId = ref(null)
const saving = ref(false)
const error = ref('')
const form = reactive({ date: '', amountYuan: '', category: '', description: '' })

const delOpen = ref(false)
const deleting = ref(false)
const deleteTarget = ref(null)

async function reload(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params = new URLSearchParams()
    params.set('page', page.value)
    params.set('page_size', pageSize.value)
    if (filters.start_date) params.set('start_date', filters.start_date)
    if (filters.end_date) params.set('end_date', filters.end_date)
    if (filters.keyword) params.set('keyword', filters.keyword)
    params.set('sort_field', sortField.value)
    params.set('sort_dir', sortDir.value)
    const data = await api('/api/records?' + params.toString())
    list.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    alert(e.message)
  } finally {
    loading.value = false
  }
}

function toggleSort(field) {
  if (sortField.value === field) {
    sortDir.value = sortDir.value === 'desc' ? 'asc' : 'desc'
  } else {
    sortField.value = field
    sortDir.value = 'desc'
  }
  reload(1)
}

function sortArrow(field) {
  if (sortField.value !== field) return ''
  return sortDir.value === 'desc' ? '↓' : '↑'
}

function resetForm() {
  form.date = today()
  form.amountYuan = ''
  form.category = ''
  form.description = ''
}

function openAdd() {
  editingId.value = null
  error.value = ''
  resetForm()
  editOpen.value = true
}

function openEdit(r) {
  editingId.value = r.id
  error.value = ''
  form.date = r.date
  form.amountYuan = centsToYuan(r.amount_cents)
  form.category = r.category || ''
  form.description = r.description || ''
  editOpen.value = true
}

async function save() {
  error.value = ''
  if (!form.date || form.amountYuan === '') {
    error.value = '请填写日期与金额'
    return
  }
  saving.value = true
  const payload = {
    date: form.date,
    amount_cents: yuanToCents(form.amountYuan),
    category: form.category.trim(),
    description: form.description.trim(),
  }
  try {
    if (editingId.value) {
      await api('/api/records/' + editingId.value, { method: 'PUT', body: JSON.stringify(payload) })
    } else {
      await api('/api/records', { method: 'POST', body: JSON.stringify(payload) })
    }
    editOpen.value = false
    await reload(page.value)
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

function askDelete(r) {
  deleteTarget.value = r
  delOpen.value = true
}

async function doDelete() {
  deleting.value = true
  try {
    await api('/api/records/' + deleteTarget.value.id, { method: 'DELETE' })
    delOpen.value = false
    await reload(page.value)
  } catch (e) {
    alert(e.message)
  } finally {
    deleting.value = false
  }
}

onMounted(() => reload(1))
</script>
