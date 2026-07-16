<template>
  <div class="min-w-0">
    <template v-if="unlimited">
      <div class="flex h-8 items-center">
        <span class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">
          {{ t('admin.dashboard.rateLimits.unlimited') }}
        </span>
      </div>
    </template>
    <template v-else-if="utilization === null || utilization === undefined">
      <div class="flex h-8 items-center text-sm text-gray-400 dark:text-gray-500">
        {{ t('admin.dashboard.rateLimits.unavailable') }}
      </div>
    </template>
    <template v-else>
      <div
        class="mb-1.5 flex items-baseline gap-2"
        :class="summary ? 'justify-between' : 'justify-end'"
      >
        <span v-if="summary" class="truncate text-sm font-semibold text-gray-900 dark:text-white">
          {{ summary }}
        </span>
        <span class="flex-shrink-0 text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ formatPercent(utilization) }}
        </span>
      </div>
      <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
        <div
          class="h-full rounded-full transition-[width] duration-300"
          :class="progressClass"
          :style="{ width: `${clampedUtilization}%` }"
        ></div>
      </div>
      <p
        v-if="resetsAt"
        class="mt-1 truncate text-[11px] text-gray-400 dark:text-gray-500"
        :title="formatDateTime(resetsAt)"
      >
        {{ t('admin.dashboard.rateLimits.resetsAt', { time: formatDateTime(resetsAt) }) }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'

const props = withDefaults(defineProps<{
  utilization?: number | null
  summary?: string
  resetsAt?: string | null
  unlimited?: boolean
}>(), {
  utilization: null,
  summary: '',
  resetsAt: null,
  unlimited: false
})

const { t } = useI18n()

const clampedUtilization = computed(() => Math.min(100, Math.max(0, props.utilization ?? 0)))
const progressClass = computed(() => {
  const value = props.utilization ?? 0
  if (value >= 90) return 'bg-red-500'
  if (value >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
})

function formatPercent(value: number): string {
  const digits = Number.isInteger(value) ? 0 : 1
  return `${value.toFixed(digits)}%`
}
</script>
