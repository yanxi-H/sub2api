<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip
} from 'chart.js'
import { Line } from 'vue-chartjs'
import type { OpsLatencyTrendPoint } from '@/api/admin/ops'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import type { ChartState } from '../types'
import { formatHistoryLabel } from '../utils/opsFormatters'

ChartJS.register(CategoryScale, Legend, LineElement, LinearScale, PointElement, Tooltip)

interface Props {
  points: OpsLatencyTrendPoint[]
  loading: boolean
  timeRange: string
}

const props = defineProps<Props>()
const { t } = useI18n()
const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const colors = computed(() => ({
  p95: '#d97706',
  p90: '#2563eb',
  p50: '#059669',
  avg: '#0891b2',
  max: '#dc2626',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}))

const hasSamples = computed(() => props.points.some(point => (point.sample_count ?? 0) > 0))

function metricValue(point: OpsLatencyTrendPoint, key: keyof OpsLatencyTrendPoint): number | null {
  const value = point[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

const chartData = computed(() => {
  if (!hasSamples.value) return null
  const definitions: Array<{ key: keyof OpsLatencyTrendPoint; label: string; color: string }> = [
    { key: 'p95_ms', label: 'P95', color: colors.value.p95 },
    { key: 'p90_ms', label: 'P90', color: colors.value.p90 },
    { key: 'p50_ms', label: 'P50', color: colors.value.p50 },
    { key: 'avg_ms', label: 'Avg', color: colors.value.avg },
    { key: 'max_ms', label: 'Max', color: colors.value.max }
  ]
  return {
    labels: props.points.map(point => formatHistoryLabel(point.bucket_start, props.timeRange)),
    datasets: definitions.map(definition => ({
      label: definition.label,
      data: props.points.map(point => metricValue(point, definition.key)),
      borderColor: definition.color,
      backgroundColor: definition.color,
      borderWidth: definition.key === 'max_ms' ? 1.5 : 2,
      borderDash: definition.key === 'max_ms' ? [5, 4] : undefined,
      tension: 0.35,
      pointRadius: 0,
      pointHitRadius: 10,
      spanGaps: false
    }))
  }
})

const state = computed<ChartState>(() => {
  if (chartData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

function formatMilliseconds(value: number): string {
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value)} ms`
}

const options = computed(() => {
  const color = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        position: 'top' as const,
        align: 'end' as const,
        labels: { color: color.text, usePointStyle: true, boxWidth: 6, font: { size: 10 } }
      },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        borderColor: color.grid,
        borderWidth: 1,
        padding: 10,
        callbacks: {
          label: (context: any) => `${context.dataset.label}: ${formatMilliseconds(context.parsed.y)}`
        }
      },
      zoom: {
        pan: { enabled: true, mode: 'x' as const, modifierKey: 'ctrl' as const },
        zoom: { wheel: { enabled: true }, pinch: { enabled: true }, mode: 'x' as const }
      }
    },
    scales: {
      x: {
        type: 'category' as const,
        grid: { display: false },
        ticks: { color: color.text, font: { size: 10 }, maxTicksLimit: 8, autoSkip: true, autoSkipPadding: 10 }
      },
      y: {
        type: 'linear' as const,
        beginAtZero: true,
        grid: { color: color.grid, borderDash: [4, 4] },
        ticks: {
          color: color.text,
          font: { size: 10 },
          callback: (value: string | number) => formatMilliseconds(Number(value))
        }
      }
    }
  }
})
</script>

<template>
  <section class="flex h-full min-h-[360px] min-w-0 flex-col rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex shrink-0 items-center gap-2">
      <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.responseTimeTrend') }}</h3>
      <HelpTooltip :content="t('admin.ops.tooltips.responseTimeTrend')" />
    </div>
    <div class="min-h-0 min-w-0 flex-1">
      <Line v-if="state === 'ready' && chartData" :data="chartData" :options="options" />
      <div v-else class="flex h-full items-center justify-center">
        <div v-if="state === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('common.loading') }}</div>
        <EmptyState v-else :title="t('common.noData')" :description="t('admin.ops.charts.emptyRequest')" />
      </div>
    </div>
  </section>
</template>
