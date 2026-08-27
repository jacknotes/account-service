<template>
  <div>
    <div class="card filter-bar" style="align-items: center">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        style="width: 280px"
      />
      <el-button type="primary" @click="load">生成报表</el-button>
      <el-button :disabled="!report" @click="exportImage">导出图片</el-button>
      <el-button :disabled="!report" @click="exportPDF">导出 PDF</el-button>
    </div>

    <div v-if="report" ref="reportContent" class="report-content">
      <h3 style="margin-top: 0">报表 {{ report.start_date }} ~ {{ report.end_date }}</h3>
      <div class="summary-cards">
        <div class="summary-card">
          <div class="label">收入</div>
          <div class="value pos">{{ formatCents(report.income_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">支出</div>
          <div class="value neg">{{ formatCents(report.expense_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">结余</div>
          <div class="value" :class="report.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(report.balance_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">笔数</div>
          <div class="value">{{ report.count }}</div>
        </div>
      </div>

      <template v-if="report.daily.length">
        <h4>收支趋势（按日）</h4>
        <div ref="dailyChartEl" class="chart-box"></div>
      </template>

      <template v-if="expenseCategories.length">
        <h4>支出分类占比</h4>
        <div ref="catChartEl" class="chart-box"></div>
      </template>

      <h4>按日统计</h4>
      <el-table :data="dailyPage" size="small" empty-text="无数据">
        <el-table-column prop="period" label="日期" width="120" />
        <el-table-column label="收入" width="130">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="130">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="结余" width="130">
          <template #default="{ row }">
            <span :class="row.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.balance_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>
      <el-pagination
        class="pager"
        background
        layout="total, prev, pager, next"
        :total="report.daily.length"
        :page-size="20"
        :current-page="dailyPageNo"
        @current-change="dailyPageNo = $event"
      />

      <h4>按月统计</h4>
      <el-table :data="report.monthly" size="small" empty-text="无数据">
        <el-table-column prop="period" label="月份" width="120" />
        <el-table-column label="收入" width="130">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="130">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="结余" width="130">
          <template #default="{ row }">
            <span :class="row.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.balance_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>

      <h4>按分类统计</h4>
      <el-table :data="catPage" size="small" empty-text="无数据">
        <el-table-column prop="category" label="分类" />
        <el-table-column label="收入" width="130">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="130">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="合计" width="130">
          <template #default="{ row }">
            <span :class="row.total_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.total_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>
      <el-pagination
        class="pager"
        background
        layout="total, prev, pager, next"
        :total="report.by_category.length"
        :page-size="20"
        :current-page="catPageNo"
        @current-change="catPageNo = $event"
      />
    </div>

    <div v-if="error" class="msg-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/http'
import { formatCents, today } from '../utils/format'
import { createChart } from '../utils/chart'

const dateRange = ref([today(), today()])
const report = ref(null)
const error = ref('')
const dailyPageNo = ref(1)
const catPageNo = ref(1)
const reportContent = ref(null)
const dailyChartEl = ref(null)
const catChartEl = ref(null)
let dailyChart = null
let catChart = null

const dailyPage = computed(() => {
  const all = report.value?.daily || []
  return all.slice((dailyPageNo.value - 1) * 20, dailyPageNo.value * 20)
})
const catPage = computed(() => {
  const all = report.value?.by_category || []
  return all.slice((catPageNo.value - 1) * 20, catPageNo.value * 20)
})
const expenseCategories = computed(() => (report.value?.by_category || []).filter((c) => c.expense_cents > 0))

async function load() {
  error.value = ''
  if (!dateRange.value || !dateRange.value[0] || !dateRange.value[1]) {
    error.value = '请选择起止日期'
    return
  }
  const [start, end] = dateRange.value
  if (start > end) {
    error.value = '开始日期不能大于结束日期'
    return
  }
  try {
    report.value = await api(`/api/report?start_date=${start}&end_date=${end}`)
    dailyPageNo.value = 1
    catPageNo.value = 1
    await nextTick()
    renderCharts()
  } catch (e) {
    error.value = e.message
  }
}

function renderCharts() {
  if (!report.value) return

  // 按日收支折线图
  if (dailyChartEl.value && report.value.daily?.length) {
    dailyChart?.destroy()
    dailyChart = createChart(dailyChartEl.value)
    dailyChart.chart.setOption({
      tooltip: { trigger: 'axis', valueFormatter: (v) => formatCents(v) },
      legend: { data: ['收入', '支出'] },
      grid: { left: 80, right: 20, top: 40, bottom: 30 },
      xAxis: { type: 'category', data: report.value.daily.map((d) => d.period) },
      yAxis: { type: 'value', axisLabel: { formatter: (v) => (v / 100).toFixed(0) } },
      series: [
        {
          name: '收入',
          type: 'line',
          smooth: true,
          data: report.value.daily.map((d) => d.income_cents),
          lineStyle: { color: '#3fd98a', width: 2 },
          itemStyle: { color: '#3fd98a' },
        },
        {
          name: '支出',
          type: 'line',
          smooth: true,
          data: report.value.daily.map((d) => d.expense_cents),
          lineStyle: { color: '#ff7b72', width: 2 },
          itemStyle: { color: '#ff7b72' },
        },
      ],
    })
  }

  // 支出分类占比环形图
  if (catChartEl.value && expenseCategories.value.length) {
    catChart?.destroy()
    catChart = createChart(catChartEl.value)
    catChart.chart.setOption({
      tooltip: {
        trigger: 'item',
        valueFormatter: (v) => formatCents(v),
      },
      legend: { bottom: 0 },
      series: [
        {
          name: '支出分类',
          type: 'pie',
          radius: ['42%', '68%'],
          data: expenseCategories.value.map((c) => ({ name: c.category || '未分类', value: Math.abs(c.expense_cents) })),
        },
      ],
    })
  }
}

onBeforeUnmount(() => {
  dailyChart?.destroy()
  catChart?.destroy()
})

async function getCanvas() {
  const html2canvas = (await import('html2canvas')).default
  // 导出背景跟随当前主题（深色 #0c0c10 / 浅色 #ffffff）
  const bg = document.body.classList.contains('theme-light') ? '#ffffff' : '#0c0c10'
  return html2canvas(reportContent.value, { backgroundColor: bg, scale: 2, useCORS: true })
}

async function exportImage() {
  try {
    const canvas = await getCanvas()
    const a = document.createElement('a')
    a.href = canvas.toDataURL('image/png')
    a.download = `报表_${dateRange.value[0]}_${dateRange.value[1]}.png`
    a.click()
  } catch (e) {
    ElMessage.error('导出图片失败: ' + e.message)
  }
}

async function exportPDF() {
  try {
    const { jsPDF } = await import('jspdf')
    const canvas = await getCanvas()
    const imgData = canvas.toDataURL('image/png')
    const pdf = new jsPDF('p', 'mm', 'a4')
    const pageWidth = pdf.internal.pageSize.getWidth()
    const pageHeight = pdf.internal.pageSize.getHeight()
    const imgWidth = pageWidth
    const imgHeight = (canvas.height * imgWidth) / canvas.width

    let heightLeft = imgHeight
    let position = 0
    pdf.addImage(imgData, 'PNG', 0, position, imgWidth, imgHeight)
    heightLeft -= pageHeight
    while (heightLeft > 0) {
      position = heightLeft - imgHeight
      pdf.addPage()
      pdf.addImage(imgData, 'PNG', 0, position, imgWidth, imgHeight)
      heightLeft -= pageHeight
    }
    pdf.save(`报表_${dateRange.value[0]}_${dateRange.value[1]}.pdf`)
  } catch (e) {
    ElMessage.error('导出 PDF 失败: ' + e.message)
  }
}
</script>
