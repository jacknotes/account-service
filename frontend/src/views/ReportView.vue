<template>
  <div>
    <div class="report-tools">
      <div class="form-row">
        <label>开始日期</label>
        <input v-model="startDate" type="date" />
      </div>
      <div class="form-row">
        <label>结束日期</label>
        <input v-model="endDate" type="date" />
      </div>
      <button class="btn btn-primary" type="button" @click="load">生成报表</button>
      <button class="btn" type="button" :disabled="!report" @click="exportPDF">导出 PDF</button>
      <button class="btn" type="button" :disabled="!report" @click="exportImage">导出图片</button>
    </div>

    <div v-if="report" class="report-content" ref="reportContent">
      <h3 style="margin-top: 0">报表 {{ report.start_date }} ~ {{ report.end_date }}</h3>
      <div class="summary-cards">
        <div class="summary-card">
          <div class="label">收入</div>
          <div class="value income">{{ formatCents(report.income_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">支出</div>
          <div class="value expense">{{ formatCents(report.expense_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">结余</div>
          <div class="value" :class="report.balance_cents >= 0 ? 'income' : 'expense'">{{ formatCents(report.balance_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">笔数</div>
          <div class="value">{{ report.count }}</div>
        </div>
      </div>

      <h4>按日统计</h4>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>日期</th>
              <th>收入</th>
              <th>支出</th>
              <th>结余</th>
              <th>笔数</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in dailyPage" :key="d.period">
              <td class="num">{{ d.period }}</td>
              <td class="num income">{{ formatCents(d.income_cents) }}</td>
              <td class="num expense">{{ formatCents(d.expense_cents) }}</td>
              <td class="num" :class="d.balance_cents >= 0 ? 'income' : 'expense'">{{ formatCents(d.balance_cents) }}</td>
              <td class="num">{{ d.count }}</td>
            </tr>
            <tr v-if="!report.daily.length"><td class="empty" colspan="5">无数据</td></tr>
          </tbody>
        </table>
      </div>
      <Pagination :page="dailyPageNo" :page-size="20" :total="report.daily.length" @change="dailyPageNo = $event" />

      <h4>按月统计</h4>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>月份</th>
              <th>收入</th>
              <th>支出</th>
              <th>结余</th>
              <th>笔数</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in report.monthly" :key="m.period">
              <td class="num">{{ m.period }}</td>
              <td class="num income">{{ formatCents(m.income_cents) }}</td>
              <td class="num expense">{{ formatCents(m.expense_cents) }}</td>
              <td class="num" :class="m.balance_cents >= 0 ? 'income' : 'expense'">{{ formatCents(m.balance_cents) }}</td>
              <td class="num">{{ m.count }}</td>
            </tr>
            <tr v-if="!report.monthly.length"><td class="empty" colspan="5">无数据</td></tr>
          </tbody>
        </table>
      </div>

      <h4>按分类统计</h4>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>分类</th>
              <th>收入</th>
              <th>支出</th>
              <th>合计</th>
              <th>笔数</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in catPage" :key="c.category">
              <td>{{ c.category }}</td>
              <td class="num income">{{ formatCents(c.income_cents) }}</td>
              <td class="num expense">{{ formatCents(c.expense_cents) }}</td>
              <td class="num" :class="c.total_cents >= 0 ? 'income' : 'expense'">{{ formatCents(c.total_cents) }}</td>
              <td class="num">{{ c.count }}</td>
            </tr>
            <tr v-if="!report.by_category.length"><td class="empty" colspan="5">无数据</td></tr>
          </tbody>
        </table>
      </div>
      <Pagination :page="catPageNo" :page-size="20" :total="report.by_category.length" @change="catPageNo = $event" />
    </div>

    <div v-if="error" class="msg-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import Pagination from '../components/Pagination.vue'
import { api } from '../api/http'
import { formatCents, today } from '../utils/format'

const startDate = ref(today())
const endDate = ref(today())
const report = ref(null)
const error = ref('')
const dailyPageNo = ref(1)
const catPageNo = ref(1)
const reportContent = ref(null)

const dailyPage = computed(() => {
  const all = report.value?.daily || []
  return all.slice((dailyPageNo.value - 1) * 20, dailyPageNo.value * 20)
})
const catPage = computed(() => {
  const all = report.value?.by_category || []
  return all.slice((catPageNo.value - 1) * 20, catPageNo.value * 20)
})

async function load() {
  error.value = ''
  if (!startDate.value || !endDate.value) {
    error.value = '请选择起止日期'
    return
  }
  if (startDate.value > endDate.value) {
    error.value = '开始日期不能大于结束日期'
    return
  }
  try {
    report.value = await api(`/api/report?start_date=${startDate.value}&end_date=${endDate.value}`)
    dailyPageNo.value = 1
    catPageNo.value = 1
  } catch (e) {
    error.value = e.message
  }
}

async function getCanvas() {
  const html2canvas = (await import('html2canvas')).default
  return html2canvas(reportContent.value, { backgroundColor: '#0f0f12', scale: 2, useCORS: true })
}

async function exportImage() {
  try {
    const canvas = await getCanvas()
    const a = document.createElement('a')
    a.href = canvas.toDataURL('image/png')
    a.download = `报表_${startDate.value}_${endDate.value}.png`
    a.click()
  } catch (e) {
    alert('导出图片失败: ' + e.message)
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
    pdf.save(`报表_${startDate.value}_${endDate.value}.pdf`)
  } catch (e) {
    alert('导出 PDF 失败: ' + e.message)
  }
}
</script>
