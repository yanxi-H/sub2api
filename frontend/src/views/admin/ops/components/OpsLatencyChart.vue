<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, BarElement, CategoryScale, Legend, LinearScale, Tooltip } from 'chart.js'
import type { ActiveElement, ChartEvent, TooltipItem } from 'chart.js'
import { Bar } from 'vue-chartjs'
import type { OpsLatencyHistogramResponse, OpsLatencyUserSummary } from '@/api/admin/ops'
import type { ChartState } from '../types'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip, Legend)

interface Props {
  latencyData: OpsLatencyHistogramResponse | null
  loading: boolean
  userId?: number | null
}

interface LatencyBucketSelection {
  title: string
  kind: 'success'
  sort: 'duration_desc'
  user_id?: number
  min_duration_ms: number
  max_duration_ms?: number
}

const props = withDefaults(defineProps<Props>(), { userId: null })
const emit = defineEmits<{
  (event: 'update:userId', userId: number | null): void
  (event: 'openDetails', selection: LatencyBucketSelection): void
}>()
const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  blue: '#2563eb',
  blueHover: '#1d4ed8',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}))

const hasData = computed(() => (props.latencyData?.total_requests ?? 0) > 0)
const state = computed<ChartState>(() => {
  if (hasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

function userLabel(user: OpsLatencyUserSummary): string {
  const identity = user.username?.trim() || user.email?.trim() || `#${user.user_id}`
  return user.deleted ? `${identity} (${t('admin.ops.userLatency.deleted')})` : identity
}

const selectableUsers = computed(() => {
  const users = new Map<number, OpsLatencyUserSummary>()
  for (const user of props.latencyData?.available_users ?? []) users.set(user.user_id, user)
  for (const user of props.latencyData?.top_avg_users ?? []) users.set(user.user_id, user)
  return [...users.values()]
})

const userOptions = computed(() => [
  { value: null, label: t('admin.ops.userLatency.allUsers') },
  ...selectableUsers.value.map((user) => ({ value: user.user_id, label: userLabel(user) }))
])

const selectedUser = computed(() => selectableUsers.value.find((user) => user.user_id === props.userId) ?? null)

function formatMs(value: number | null | undefined): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${new Intl.NumberFormat().format(Math.round(value))} ms`
}

function updateUser(value: string | number | boolean | null) {
  emit('update:userId', typeof value === 'number' && value > 0 ? value : null)
}

const chartData = computed(() => {
  if (!props.latencyData || !hasData.value) return null
  const c = colors.value
  return {
    labels: props.latencyData.buckets.map((bucket) => bucket.range),
    datasets: [
      {
        label: t('admin.ops.requests'),
        data: props.latencyData.buckets.map((bucket) => bucket.count),
        backgroundColor: c.blue,
        hoverBackgroundColor: c.blueHover,
        borderRadius: 3,
        barPercentage: 0.68
      }
    ]
  }
})

const options = computed(() => {
  const c = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    onClick: (_event: ChartEvent, elements: ActiveElement[]) => {
      const index = elements[0]?.index
      const bucket = typeof index === 'number' ? props.latencyData?.buckets[index] : null
      if (!bucket) return
      emit('openDetails', {
        title: t('admin.ops.userLatency.bucketDetails', { range: bucket.range }),
        kind: 'success',
        sort: 'duration_desc',
        user_id: props.userId ?? undefined,
        min_duration_ms: bucket.min_ms,
        max_duration_ms: bucket.max_ms ?? undefined
      })
    },
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        callbacks: {
          label: (context: TooltipItem<'bar'>) => {
            const count = Number(context.parsed.y ?? 0)
            const total = props.latencyData?.total_requests ?? 0
            const share = total > 0 ? ((count / total) * 100).toFixed(1) : '0.0'
            return `${t('admin.ops.requests')}: ${new Intl.NumberFormat().format(count)} (${share}%)`
          }
        }
      }
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: c.text, font: { size: 10 }, maxRotation: 0, autoSkip: false }
      },
      y: {
        beginAtZero: true,
        grid: { color: c.grid },
        ticks: { color: c.text, font: { size: 10 }, precision: 0 }
      }
    }
  }
})
</script>

<template>
  <section class="flex h-full min-h-[360px] flex-col rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
          {{ t('admin.ops.userLatency.title') }}
          <HelpTooltip :content="t('admin.ops.tooltips.userLatency')" />
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.userLatency.summary', { count: latencyData?.total_requests ?? 0, avg: formatMs(latencyData?.avg_duration_ms) }) }}
        </p>
      </div>
      <Select
        class="w-full sm:w-56"
        :model-value="userId"
        :options="userOptions"
        :searchable="true"
        :search-placeholder="t('admin.ops.userLatency.searchUser')"
        @update:model-value="updateUser"
      />
    </div>

    <div v-if="(latencyData?.top_avg_users?.length ?? 0) > 0" class="mt-3 flex flex-wrap items-center gap-2" data-testid="latency-top-users">
      <span class="text-[11px] font-semibold text-gray-500 dark:text-gray-400">{{ t('admin.ops.userLatency.topAvg') }}</span>
      <button
        v-for="(user, index) in latencyData?.top_avg_users ?? []"
        :key="user.user_id"
        type="button"
        :class="[
          'inline-flex max-w-full items-center gap-1.5 rounded-md border px-2 py-1 text-[11px] transition-colors',
          user.user_id === userId
            ? 'border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-950/40 dark:text-blue-300'
            : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700'
        ]"
        @click="emit('update:userId', user.user_id)"
      >
        <span class="font-bold">{{ index + 1 }}</span>
        <span class="max-w-28 truncate">{{ userLabel(user) }}</span>
        <span class="font-mono font-semibold">{{ formatMs(user.avg_duration_ms) }}</span>
      </button>
    </div>

    <div class="mt-4 min-h-0 flex-1">
      <Bar v-if="state === 'ready' && chartData" :data="chartData" :options="options" />
      <div v-else class="flex h-full items-center justify-center">
        <div v-if="state === 'loading'" class="animate-pulse text-sm text-gray-400">{{ t('common.loading') }}</div>
        <EmptyState
          v-else
          :title="t('common.noData')"
          :description="selectedUser ? t('admin.ops.userLatency.emptyUser') : t('admin.ops.charts.emptyRequest')"
        />
      </div>
    </div>
  </section>
</template>
