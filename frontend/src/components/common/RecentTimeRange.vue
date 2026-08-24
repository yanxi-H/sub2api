<template>
  <div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
    <div
      class="flex max-w-full overflow-x-auto rounded-md border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-600 dark:bg-dark-800"
      role="group"
      :aria-label="t('usage.recentRange')"
    >
      <button
        v-for="option in options"
        :key="option.minutes"
        type="button"
        class="shrink-0 rounded px-2.5 py-1.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
        :class="activeMinutes === option.minutes
          ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400'
          : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'"
        :aria-pressed="activeMinutes === option.minutes"
        @click="$emit('select', option.minutes)"
      >
        {{ option.label }}
      </button>
    </div>
    <span
      v-if="startTime && endTime"
      class="whitespace-nowrap text-xs text-gray-500 dark:text-gray-400"
      data-testid="exact-time-range"
    >
      {{ formatTime(startTime) }} - {{ formatTime(endTime) }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps<{
  activeMinutes: number | null
  startTime?: string
  endTime?: string
}>()

defineEmits<{
  select: [minutes: number]
}>()

const i18n = useI18n()
const { t } = i18n

const options = computed(() => [
  { minutes: 1, label: t('usage.recentMinutes', { count: 1 }) },
  { minutes: 5, label: t('usage.recentMinutes', { count: 5 }) },
  { minutes: 10, label: t('usage.recentMinutes', { count: 10 }) },
  { minutes: 30, label: t('usage.recentHalfHour') },
  { minutes: 180, label: t('usage.recentHours', { count: 3 }) },
  { minutes: 360, label: t('usage.recentHours', { count: 6 }) },
])

const formatTime = (value: string) => new Intl.DateTimeFormat(i18n.locale?.value, {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
}).format(new Date(value))
</script>
