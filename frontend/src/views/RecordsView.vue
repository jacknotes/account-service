<template>
  <div>
    <!-- 本月结余横幅 -->
    <div class="balance-banner">
      <div class="balance-main">
        <div class="balance-label">本月结余</div>
        <div class="balance-value">{{ formatCents(monthSummary.balance_cents) }}</div>
        <div class="balance-sub">
          <span>收入 <b class="pos">{{ formatCents(monthSummary.income_cents) }}</b></span>
          <span>支出 <b class="neg">{{ formatCents(monthSummary.expense_cents) }}</b></span>
        </div>
      </div>
      <el-button type="primary" size="large" @click="openAdd">＋ 记一笔</el-button>
    </div>

    <!-- 筛选栏 -->
    <div class="card filter-bar" style="align-items: center">
      <div class="quick-pills">
        <button
          v-for="q in quickRanges"
          :key="q.key"
          type="button"
          class="pill"
          :class="{ active: quick === q.key }"
          @click="applyQuick(q.key)"
        >
          {{ q.label }}
        </button>
      </div>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        style="width: 260px"
        @change="onDateChange"
      />
      <el-input
        v-model="filters.keyword"
        placeholder="搜索描述/分类"
        clearable
        style="width: 200px"
        @keyup.enter="reload(1)"
        @clear="reload(1)"
      />
      <el-button type="primary" @click="reload(1)">查询</el-button>
    </div>

    <!-- 桌面表格 -->
    <div class="card desktop-table">
      <el-table :data="list" v-loading="loading" empty-text="暂无记录" @sort-change="onSortChange">
        <el-table-column prop="date" label="日期" sortable="custom" width="120" />
        <el-table-column prop="amount" label="金额" sortable="custom" width="130">
          <template #default="{ row }">
            <span class="num" :class="row.amount_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.amount_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" sortable="custom" width="130">
          <template #default="{ row }">
            <el-tag :type="row.amount_cents >= 0 ? 'success' : 'warning'" effect="dark" size="small">
              {{ row.category || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-button size="small" text @click="openEdit(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
          </template>
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

    <!-- 移动端卡片列表 -->
    <div class="record-cards">
      <div v-if="!list.length" class="empty-tip">{{ loading ? '加载中...' : '暂无记录' }}</div>
      <div v-for="r in list" :key="r.id" class="record-card">
        <div class="rc-main">
          <el-tag :type="r.amount_cents >= 0 ? 'success' : 'warning'" effect="dark" size="small">
            {{ r.category || '未分类' }}
          </el-tag>
          <span class="rc-desc">{{ r.description || '-' }}</span>
        </div>
        <div class="rc-amount" :class="r.amount_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(r.amount_cents) }}</div>
        <div class="rc-foot">
          <span>{{ r.date }}</span>
          <span>
            <el-button size="small" text @click="openEdit(r)">编辑</el-button>
            <el-button size="small" text type="danger" @click="askDelete(r)">删除</el-button>
          </span>
        </div>
      </div>
      <el-pagination
        class="pager"
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="reload"
      />
    </div>

    <!-- 记一笔 / 编辑 -->
    <el-dialog v-model="editOpen" :title="editingId ? '编辑记录' : '记一笔'" width="440px">
      <el-form label-width="80px">
        <el-form-item label="类型">
          <el-radio-group v-model="form.type">
            <el-radio-button value="expense">支出</el-radio-button>
            <el-radio-button value="income">收入</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="form.date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="金额（元）">
          <el-input-number v-model="form.amountYuan" :min="0.01" :precision="2" controls-position="right" style="width: 100%" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.category" filterable placeholder="选择分类" style="width: 100%">
            <el-option v-for="c in typeCategories" :key="c.id" :label="c.name" :value="c.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" maxlength="255" placeholder="备注" />
        </el-form-item>
      </el-form>
      <div class="msg-error" v-if="error">{{ error }}</div>
      <template #footer>
        <el-button @click="editOpen = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api/http'
import { formatCents, yuanToCents, today, monthRange, prevMonthRange } from '../utils/format'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const filters = reactive({ keyword: '' })
const sortField = ref('date')
const sortDir = ref('desc')

// ---- 默认当月 + 快捷切换 ----
const quick = ref('month') // 'month' | 'last' | 'all' | ''（自定义范围）
const quickRanges = [
  { key: 'month', label: '本月' },
  { key: 'last', label: '上月' },
  { key: 'all', label: '全部' },
]
const dateRange = ref(monthRange())

function applyQuick(key) {
  quick.value = key
  if (key === 'month') dateRange.value = monthRange()
  else if (key === 'last') dateRange.value = prevMonthRange()
  else dateRange.value = null
  reload(1)
}

// 手动改日期范围后取消快捷胶囊选中态
function onDateChange() {
  quick.value = ''
  reload(1)
}

// ---- 本月结余横幅 ----
const monthSummary = reactive({ income_cents: 0, expense_cents: 0, balance_cents: 0 })
async function loadMonthSummary() {
  try {
    const now = new Date()
    const s = await api(`/api/summary/monthly?year=${now.getFullYear()}&month=${now.getMonth() + 1}`)
    monthSummary.income_cents = s.income_cents || 0
    monthSummary.expense_cents = s.expense_cents || 0
    monthSummary.balance_cents = s.balance_cents || 0
  } catch {
    /* 横幅加载失败不阻塞列表 */
  }
}

// ---- 分类（记一笔下拉选项）----
const categories = ref([])
async function loadCategories() {
  try {
    const data = await api('/api/categories')
    categories.value = data.data || []
  } catch {
    /* 分类加载失败不阻塞列表 */
  }
}

// ---- 列表 ----
async function reload(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params = new URLSearchParams()
    params.set('page', page.value)
    params.set('page_size', pageSize.value)
    if (dateRange.value && dateRange.value[0]) params.set('start_date', dateRange.value[0])
    if (dateRange.value && dateRange.value[1]) params.set('end_date', dateRange.value[1])
    if (filters.keyword) params.set('keyword', filters.keyword)
    params.set('sort_field', sortField.value)
    params.set('sort_dir', sortDir.value)
    const data = await api('/api/records?' + params.toString())
    list.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function onSortChange({ prop, order }) {
  if (!order) {
    sortField.value = 'date'
    sortDir.value = 'desc'
  } else {
    sortField.value = prop || 'date'
    sortDir.value = order === 'ascending' ? 'asc' : 'desc'
  }
  reload(1)
}

// ---- 记一笔 / 编辑 ----
const editOpen = ref(false)
const editingId = ref(null)
const saving = ref(false)
const error = ref('')
const form = reactive({ type: 'expense', date: '', amountYuan: undefined, category: '', description: '' })

const typeCategories = computed(() => categories.value.filter((c) => c.type === form.type))

function openAdd() {
  editingId.value = null
  error.value = ''
  Object.assign(form, { type: 'expense', date: today(), amountYuan: undefined, category: '', description: '' })
  editOpen.value = true
}

function openEdit(r) {
  editingId.value = r.id
  error.value = ''
  Object.assign(form, {
    type: r.amount_cents >= 0 ? 'income' : 'expense',
    date: r.date,
    amountYuan: Math.abs(r.amount_cents) / 100,
    category: r.category || '',
    description: r.description || '',
  })
  editOpen.value = true
}

async function save() {
  error.value = ''
  if (!form.date || !form.amountYuan) {
    error.value = '请填写日期与金额'
    return
  }
  saving.value = true
  const wasEdit = !!editingId.value
  const cents = yuanToCents(form.amountYuan)
  const payload = {
    date: form.date,
    // 金额输入为正数，保存时支出取负（与 amount_cents 语义一致）
    amount_cents: form.type === 'expense' ? -cents : cents,
    category: (form.category || '').trim(),
    description: (form.description || '').trim(),
  }
  try {
    if (wasEdit) {
      await api('/api/records/' + editingId.value, { method: 'PUT', body: JSON.stringify(payload) })
    } else {
      await api('/api/records', { method: 'POST', body: JSON.stringify(payload) })
    }
    editOpen.value = false
    ElMessage.success(wasEdit ? '已更新' : '已记一笔')
    await Promise.all([reload(page.value), loadMonthSummary()])
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function askDelete(r) {
  try {
    await ElMessageBox.confirm('确定删除该记录吗？此操作不可撤销。', '删除记录', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await api('/api/records/' + r.id, { method: 'DELETE' })
    ElMessage.success('已删除')
    await Promise.all([reload(page.value), loadMonthSummary()])
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(() => {
  reload(1)
  loadMonthSummary()
  loadCategories()
})
</script>
