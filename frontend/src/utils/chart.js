// ECharts 公共封装：按需注册 + 深浅双主题（跟随 body.theme-light）+ 窗口 resize 自适应
import * as echarts from 'echarts/core'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

// 风格 C 图表主题（深色底 + 金色主色）
echarts.registerTheme('gold-dark', {
  backgroundColor: 'transparent',
  textStyle: { color: '#c8c8d4' },
  color: ['#f5c451', '#3fd98a', '#ff7b72', '#e8930c', '#6fb3ff', '#b48ef7'],
  legend: { textStyle: { color: '#8b8b98' } },
  categoryAxis: {
    axisLine: { lineStyle: { color: '#2e2e3c' } },
    axisLabel: { color: '#8b8b98' },
    splitLine: { show: false },
  },
  valueAxis: {
    axisLine: { lineStyle: { color: '#2e2e3c' } },
    axisLabel: { color: '#8b8b98' },
    splitLine: { lineStyle: { color: '#1e1e28' } },
  },
})

// 浅色主题（与 body.theme-light 令牌一致）
echarts.registerTheme('gold-light', {
  backgroundColor: 'transparent',
  textStyle: { color: '#3a3a44' },
  color: ['#d99a17', '#1f9e63', '#d6453d', '#b57f10', '#4a7fb5', '#8a6bb8'],
  legend: { textStyle: { color: '#6b6b78' } },
  categoryAxis: {
    axisLine: { lineStyle: { color: '#dcdfe6' } },
    axisLabel: { color: '#6b6b78' },
    splitLine: { show: false },
  },
  valueAxis: {
    axisLine: { lineStyle: { color: '#dcdfe6' } },
    axisLabel: { color: '#6b6b78' },
    splitLine: { lineStyle: { color: '#ececef' } },
  },
})

// 按当前页面主题返回图表主题名
function currentTheme() {
  return document.body.classList.contains('theme-light') ? 'gold-light' : 'gold-dark'
}

// createChart 初始化图表并监听窗口 resize；返回 { chart, destroy }
export function createChart(el) {
  const chart = echarts.init(el, currentTheme())
  const onResize = () => chart.resize()
  window.addEventListener('resize', onResize)
  return {
    chart,
    destroy() {
      window.removeEventListener('resize', onResize)
      chart.dispose()
    },
  }
}
