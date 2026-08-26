<template>
  <div>
    <div class="card">
      <el-tabs v-model="mode" @tab-change="load">
        <el-tab-pane label="每日" name="daily" />
        <el-tab-pane label="每月" name="monthly" />
        <el-tab-pane label="每年" name="yearly" />
      </el-tabs>

      <div class="filter-bar">
        <template v-if="mode === 'daily'">
          <el-date-picker v-model="dailyDate" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" @change="load" />
        </template>
        <template v-else-if="mode === 'monthly'">
          <el-date-picker v-model="monthValue" type="month" value-format="YYYY-MM" placeholder="选择月份" @change="onMonthChange" />
        </template>
        <template v-else>
          <el-input-number v-model="year" :min="2000" :max="2100" @change="load" />
        </template>
      </div>

      <div v-if="summary" class="summary-cards">
        <div class="summary-card">
          <div class="label">收入</div>
          <div class="value pos">{{ formatCents(summary.income_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">支出</div>
          <div class="value neg">{{ formatCents(summary.expense_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">结余</div>
          <div class="value" :class="summary.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(summary.balance_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">笔数</div>
          <div class="value">{{ summary.count }}</div>
        </div>
      </div>

      <!-- 每日模式：当日累计结余迷你趋势图 -->
      <div v-show="hasDailyRecords" ref="trendEl" class="mini-chart"></div>

      <!-- 每日明细 -->
      <el-table v-if="mode === 'daily'" :data="summary?.records || []" empty-text="暂无记录">
        <el-table-column label="金额" width="140">
          <template #default="{ row }">
            <span class="num" :class="row.amount_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.amount_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="分类" width="140">
          <template #default="{ row }">
            <el-tag :type="row.amount_cents >= 0 ? 'success' : 'warning'" effect="dark" size="small">{{ row.category || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" />
      </el-table>

      <!-- 月/年分项 -->
      <el-table v-if="mode !== 'daily'" :data="summary?.breakdown || []" empty-text="暂无数据">
        <el-table-column prop="period" :label="mode === 'monthly' ? '日期' : '月份'" width="120" />
        <el-table-column label="收入" width="140">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="140">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="结余" width="140">
          <template #default="{ row }">
            <span :class="row.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.balance_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>
    </div>
    <div v-if="error" class="msg-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api/http'
import { formatCents, today } from '../utils/format'
import { createChart } from '../utils/chart'

const mode = ref('daily')
const dailyDate = ref(today())
const year = ref(new Date().getFullYear())
const month = ref(new Date().getMonth() + 1)
const monthValue = ref(`${year.value}-${String(month.value).padStart(2, '0')}`)
const summary = ref(null)
const error = ref('')

const hasDailyRecords = computed(() => mode.value === 'daily' && !!(summary.value?.records?.length))

function onMonthChange(v) {
  if (!v) return
  const [y, m] = v.split('-').map(Number)
  year.value = y
  month.value = m
  load()
}

async function load() {
  error.value = ''
  summary.value = null
  try {
    if (mode.value === 'daily') {
      summary.value = await api('/api/summary/daily?date=' + dailyDate.value)
    } else if (mode.value === 'monthly') {
      summary.value = await api(`/api/summary/monthly?year=${year.value}&month=${month.value}`)
    } else {
      summary.value = await api('/api/summary/yearly?year=' + year.value)
    }
  } catch (e) {
    error.value = e.message
  }
}

// ---- 迷你趋势图：当日每笔记录后的累计结余走势 ----
const trendEl = ref(null)
let trendChart = null

function renderTrend() {
  const recs = summary.value?.records
  if (!trendEl.value || !recs?.length) return
  trendChart?.destroy()
  trendChart = createChart(trendEl.value)
  let acc = 0
  const data = recs.map((r) => {
    acc += r.amount_cents
    return acc
  })
  trendChart.chart.setOption({
    grid: { left: 80, right: 16, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis', valueFormatter: (v) => formatCents(v) },
    xAxis: { type: 'category', data: recs.map((_, i) => `第${i + 1}笔`) },
    yAxis: { type: 'value', axisLabel: { formatter: (v) => (v / 100).toFixed(0) } },
    series: [
      {
        name: '累计结余',
        type: 'line',
        smooth: true,
        data,
        lineStyle: { color: '#f5c451', width: 2 },
        itemStyle: { color: '#f5c451' },
        areaStyle: { color: 'rgba(245,196,81,0.12)' },
      },
    ],
  })
}

watch(hasDailyRecords, (v) => {
  if (v) nextTick(renderTrend)
})
onBeforeUnmount(() => trendChart?.destroy())

onMounted(load)
</script>
