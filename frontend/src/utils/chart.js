// ECharts 公共封装：按需注册 + 金色深色主题 + 窗口 resize 自适应
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

// createChart 初始化图表并监听窗口 resize；返回 { chart, destroy }
export function createChart(el) {
  const chart = echarts.init(el, 'gold-dark')
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
