<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js'
import { Bar } from 'vue-chartjs'
import type { OpsPerformanceDiagnosticsResponse } from '@/api/admin/ops'
import { chartPalette, formatAxisTime, formatDateTime } from './monitorCenterUtils'

ChartJS.register(CategoryScale, LinearScale, BarElement, Legend, Tooltip)

const props = defineProps<{
  data: OpsPerformanceDiagnosticsResponse | null
  loading: boolean
  range: string
}>()
const { t } = useI18n()
const colors = ['#c98524', '#c84d49', '#3178c6', '#4f9d73', '#7569b7', '#35889d', '#b06b91', '#6f8f3d', '#d46645', '#71879d']

function causeLabel(cause: string): string {
  const key = `admin.ops.performance.causes.${cause}`
  const translated = t(key)
  return translated === key ? cause : translated
}

const causes = computed(() => {
  const totals = new Map<string, number>()
  for (const point of props.data?.trend ?? []) {
    for (const [cause, count] of Object.entries(point.causes ?? {})) totals.set(cause, (totals.get(cause) ?? 0) + count)
  }
  return [...totals.entries()].sort((a, b) => b[1] - a[1]).map(([cause]) => cause)
})
const hasData = computed(() => causes.value.length > 0 && (props.data?.trend?.length ?? 0) > 0)
const chartData = computed(() => ({
  labels: (props.data?.trend ?? []).map((point) => formatAxisTime(point.bucket_start, props.range === '24h' || props.range === 'custom')),
  datasets: causes.value.map((cause, index) => ({
    label: causeLabel(cause),
    data: (props.data?.trend ?? []).map((point) => point.causes?.[cause] ?? 0),
    backgroundColor: colors[index % colors.length],
    borderWidth: 0,
    stack: 'causes',
    barPercentage: .86,
    categoryPercentage: .9,
  })),
}))
const chartOptions = computed(() => {
  const palette = chartPalette()
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: { position: 'top' as const, align: 'end' as const, labels: { color: palette.text, usePointStyle: true, boxWidth: 7, padding: 12, font: { size: 9 } } },
      tooltip: {
        backgroundColor: palette.tooltipBg, borderColor: palette.tooltipBorder, borderWidth: 1,
        titleColor: palette.title, bodyColor: palette.text, padding: 10,
        callbacks: {
          title: (items: any[]) => {
            const index = items[0]?.dataIndex
            return typeof index === 'number' ? formatDateTime(props.data?.trend?.[index]?.bucket_start) : ''
          },
        },
      },
    },
    scales: {
      x: { stacked: true, grid: { display: false }, ticks: { color: palette.text, maxTicksLimit: 9, autoSkip: true, font: { size: 9 } } },
      y: { stacked: true, beginAtZero: true, grid: { color: palette.grid }, ticks: { color: palette.text, precision: 0, font: { size: 9 } } },
    },
  }
})
</script>

<template>
  <div class="mc-slow-chart">
    <Bar v-if="hasData" :data="chartData" :options="chartOptions" />
    <div v-else class="mc-empty">{{ loading ? t('common.loading') : t('admin.monitorCenter.slow.noSlowRequests') }}</div>
  </div>
</template>

<style scoped>
.mc-slow-chart { height: 230px; min-width: 0; border-top: 1px solid var(--mc-line); padding-top: 10px; }
</style>
