<template>
  <div>
    <div class="tabs">
      <button class="tab" :class="{ active: mode === 'daily' }" @click="switchMode('daily')">每日</button>
      <button class="tab" :class="{ active: mode === 'monthly' }" @click="switchMode('monthly')">每月</button>
      <button class="tab" :class="{ active: mode === 'yearly' }" @click="switchMode('yearly')">每年</button>
    </div>

    <div class="card">
      <div class="filter-bar">
        <template v-if="mode === 'daily'">
          <div class="form-row">
            <label>日期</label>
            <input v-model="dailyDate" type="date" @change="load" />
          </div>
        </template>
        <template v-else-if="mode === 'monthly'">
          <div class="form-row">
            <label>年份</label>
            <input v-model.number="year" type="number" min="2000" max="2100" @change="load" />
          </div>
          <div class="form-row">
            <label>月份</label>
            <select v-model.number="month" @change="load">
              <option v-for="m in 12" :key="m" :value="m">{{ m }} 月</option>
            </select>
          </div>
        </template>
        <template v-else>
          <div class="form-row">
            <label>年份</label>
            <input v-model.number="year" type="number" min="2000" max="2100" @change="load" />
          </div>
        </template>
      </div>

      <div v-if="summary" class="summary-cards">
        <div class="summary-card">
          <div class="label">收入</div>
          <div class="value income">{{ formatCents(summary.income_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">支出</div>
          <div class="value expense">{{ formatCents(summary.expense_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">结余</div>
          <div class="value" :class="summary.balance_cents >= 0 ? 'income' : 'expense'">
            {{ formatCents(summary.balance_cents) }}
          </div>
        </div>
        <div class="summary-card">
          <div class="label">笔数</div>
          <div class="value">{{ summary.count }}</div>
        </div>
      </div>

      <!-- 每日明细 -->
      <div v-if="mode === 'daily' && summary && summary.records && summary.records.length" class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>金额</th>
              <th>分类</th>
              <th>描述</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in summary.records" :key="r.id">
              <td class="num" :class="r.amount_cents >= 0 ? 'income' : 'expense'">{{ formatCents(r.amount_cents) }}</td>
              <td>{{ r.category || '-' }}</td>
              <td>{{ r.description || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 月/年分项 -->
      <div v-if="mode !== 'daily' && summary && summary.breakdown && summary.breakdown.length" class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>{{ mode === 'monthly' ? '日期' : '月份' }}</th>
              <th>收入</th>
              <th>支出</th>
              <th>结余</th>
              <th>笔数</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="b in summary.breakdown" :key="b.period">
              <td class="num">{{ b.period }}</td>
              <td class="num income">{{ formatCents(b.income_cents) }}</td>
              <td class="num expense">{{ formatCents(b.expense_cents) }}</td>
              <td class="num" :class="b.balance_cents >= 0 ? 'income' : 'expense'">{{ formatCents(b.balance_cents) }}</td>
              <td class="num">{{ b.count }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="!summary" class="msg-error">{{ error }}</div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/http'
import { formatCents, today } from '../utils/format'

const mode = ref('daily')
const dailyDate = ref(today())
const year = ref(new Date().getFullYear())
const month = ref(new Date().getMonth() + 1)
const summary = ref(null)
const error = ref('')

function switchMode(m) {
  mode.value = m
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

onMounted(load)
</script>
