<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar } from 'vue-chartjs'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js'
import type { ActiveElement, ChartEvent, TooltipItem } from 'chart.js'
import type { OpsUserErrorDistributionItem, OpsUserErrorDistributionResponse } from '@/api/admin/ops'
import type { ChartState } from '../types'
import EmptyState from '@/components/common/EmptyState.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip, Legend)

interface Props {
  data: OpsUserErrorDistributionResponse | null
  loading: boolean
}

interface ErrorSelection {
  userId?: number
  errorTypes?: string[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (event: 'openDetails', selection?: ErrorSelection): void
}>()
const { t } = useI18n()

const palette = ['#dc2626', '#f59e0b', '#2563eb', '#7c3aed', '#0891b2', '#16a34a', '#6b7280']
const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const hasData = computed(() => (props.data?.items?.length ?? 0) > 0 && (props.data?.total ?? 0) > 0)
const state = computed<ChartState>(() => {
  if (hasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

function userLabel(user: OpsUserErrorDistributionItem): string {
  if (user.user_id == null) return t('admin.ops.userErrors.unknownUser')
  const identity = user.username?.trim() || user.email?.trim() || `#${user.user_id}`
  return user.deleted ? `${identity} (${t('admin.ops.userLatency.deleted')})` : identity
}

function errorTypeLabel(type: string): string {
  if (type === 'other') return t('admin.ops.other')
  if (type === 'unknown') return t('admin.ops.userErrors.unknownType')
  return type.split('_').join(' ')
}

const errorTypes = computed(() => {
  const totals = new Map<string, number>()
  for (const user of props.data?.items ?? []) {
    for (const item of user.errors ?? []) {
      totals.set(item.error_type, (totals.get(item.error_type) ?? 0) + Number(item.count || 0))
    }
  }
  return [...totals.entries()]
    .sort(([leftType, left], [rightType, right]) => {
      if (leftType === 'other') return 1
      if (rightType === 'other') return -1
      return right - left || leftType.localeCompare(rightType)
    })
    .map(([type]) => type)
})

const chartData = computed(() => {
  if (!hasData.value || !props.data) return null
  const users = props.data.items
  return {
    labels: users.map(userLabel),
    datasets: errorTypes.value.map((type, index) => ({
      label: errorTypeLabel(type),
      errorType: type,
      data: users.map((user) => user.errors.find((item) => item.error_type === type)?.count ?? 0),
      backgroundColor: palette[index % palette.length],
      borderWidth: 0,
      borderRadius: 2,
      barPercentage: 0.72
    }))
  }
})

const options = computed(() => ({
  indexAxis: 'y' as const,
  responsive: true,
  maintainAspectRatio: false,
  onClick: (_event: ChartEvent, elements: ActiveElement[]) => {
    const element = elements[0]
    if (!element) return
    const user = props.data?.items[element.index]
    const errorType = errorTypes.value[element.datasetIndex]
    if (!user || !errorType) return
    emit('openDetails', {
      userId: user.user_id ?? undefined,
      errorTypes: errorType === 'other' ? undefined : [errorType]
    })
  },
  plugins: {
    legend: {
      display: true,
      position: 'bottom' as const,
      labels: {
        color: isDarkMode.value ? '#d1d5db' : '#4b5563',
        boxWidth: 10,
        boxHeight: 10,
        padding: 12,
        font: { size: 10 }
      }
    },
    tooltip: {
      backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
      titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
      bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
      callbacks: {
        label: (context: TooltipItem<'bar'>) => {
          const count = Number(context.parsed.x ?? 0)
          const userTotal = props.data?.items[context.dataIndex]?.total ?? 0
          const share = userTotal > 0 ? ((count / userTotal) * 100).toFixed(1) : '0.0'
          const type = errorTypes.value[context.datasetIndex]
          return `${errorTypeLabel(type)}: ${new Intl.NumberFormat().format(count)} (${share}%)`
        },
        footer: (items: TooltipItem<'bar'>[]) => {
          const total = props.data?.items[items[0]?.dataIndex]?.total ?? 0
          return t('admin.ops.userErrors.userTotal', { count: new Intl.NumberFormat().format(total) })
        }
      }
    }
  },
  scales: {
    x: {
      stacked: true,
      beginAtZero: true,
      grid: { color: isDarkMode.value ? '#374151' : '#f3f4f6' },
      ticks: { color: isDarkMode.value ? '#9ca3af' : '#6b7280', precision: 0, font: { size: 10 } }
    },
    y: {
      stacked: true,
      grid: { display: false },
      ticks: { color: isDarkMode.value ? '#d1d5db' : '#4b5563', font: { size: 10 } }
    }
  }
}))
</script>

<template>
  <section class="flex h-full min-h-[360px] flex-col rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
          {{ t('admin.ops.userErrors.title') }}
          <HelpTooltip :content="t('admin.ops.tooltips.userErrors')" />
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.userErrors.summary', { users: data?.total_users ?? 0, errors: data?.total ?? 0 }) }}
        </p>
      </div>
      <button
        type="button"
        class="inline-flex items-center rounded-md border border-gray-200 px-2 py-1 text-[11px] font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700"
        :disabled="state !== 'ready'"
        @click="emit('openDetails')"
      >
        {{ t('admin.ops.requestDetails.details') }}
      </button>
    </div>

    <p v-if="(data?.total_users ?? 0) > (data?.user_limit ?? 0)" class="mt-2 text-[11px] text-gray-400">
      {{ t('admin.ops.userErrors.topLimit', { count: data?.user_limit ?? 0 }) }}
    </p>

    <div class="mt-3 min-h-0 flex-1">
      <Bar v-if="state === 'ready' && chartData" :data="chartData" :options="options" />
      <div v-else class="flex h-full items-center justify-center">
        <div v-if="state === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('common.loading') }}</div>
        <EmptyState v-else :title="t('common.noData')" :description="t('admin.ops.charts.emptyError')" />
      </div>
    </div>
  </section>
</template>
