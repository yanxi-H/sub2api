<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowUp, ArrowUpDown } from '@lucide/vue'
import type { OpsPerformanceImpact } from '@/api/admin/ops'
import { formatMs, formatPercent } from './monitorCenterUtils'

type Dimension = 'user' | 'account' | 'model'
type SortKey = 'name' | 'request_count' | 'slow_rate' | 'e2e_p95_ms' | 'ttft_p95_ms' | 'queue_p95_ms' | 'main_cause'
type SortDirection = 'asc' | 'desc'

interface Column {
  key: SortKey
  label: string
  numeric?: boolean
}

const props = defineProps<{ impacts: OpsPerformanceImpact[] }>()
const { t } = useI18n()
const dimension = ref<Dimension>('user')
const sortKey = ref<SortKey | null>(null)
const sortDirection = ref<SortDirection>('asc')

const dimensions: Dimension[] = ['user', 'account', 'model']
const columns = computed<Column[]>(() => [
  { key: 'name', label: t(`admin.monitorCenter.slow.dimensions.${dimension.value}`) },
  { key: 'request_count', label: t('admin.monitorCenter.slow.requests'), numeric: true },
  { key: 'slow_rate', label: t('admin.monitorCenter.slow.slowRate'), numeric: true },
  { key: 'e2e_p95_ms', label: 'E2E P95', numeric: true },
  { key: 'ttft_p95_ms', label: 'TTFT P95', numeric: true },
  { key: 'queue_p95_ms', label: t('admin.monitorCenter.slow.queueP95'), numeric: true },
  { key: 'main_cause', label: t('admin.monitorCenter.slow.mainCause') },
])
const dimensionCounts = computed<Record<Dimension, number>>(() => ({
  user: props.impacts.filter((item) => item.dimension === 'user').length,
  account: props.impacts.filter((item) => item.dimension === 'account').length,
  model: props.impacts.filter((item) => item.dimension === 'model').length,
}))
const rows = computed(() => {
  const filtered = props.impacts.filter((item) => item.dimension === dimension.value)
  if (!sortKey.value) return filtered

  const key = sortKey.value
  return filtered
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      const leftValue = sortValue(left.item, key)
      const rightValue = sortValue(right.item, key)
      const leftMissing = leftValue === null || leftValue === undefined || leftValue === ''
      const rightMissing = rightValue === null || rightValue === undefined || rightValue === ''
      if (leftMissing !== rightMissing) return leftMissing ? 1 : -1
      if (leftMissing && rightMissing) return left.index - right.index

      const comparison = typeof leftValue === 'number' && typeof rightValue === 'number'
        ? leftValue - rightValue
        : String(leftValue).localeCompare(String(rightValue), undefined, { numeric: true, sensitivity: 'base' })
      return (sortDirection.value === 'asc' ? comparison : -comparison) || left.index - right.index
    })
    .map(({ item }) => item)
})

function sortValue(item: OpsPerformanceImpact, key: SortKey): string | number | null | undefined {
  if (key === 'name') return item.name || `#${item.id}`
  if (key === 'main_cause') return causeLabel(item.main_cause)
  return item[key]
}

function toggleSort(key: SortKey): void {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDirection.value = 'asc'
}

function ariaSort(key: SortKey): 'ascending' | 'descending' | 'none' {
  if (sortKey.value !== key) return 'none'
  return sortDirection.value === 'asc' ? 'ascending' : 'descending'
}

function causeLabel(cause: string): string {
  const key = `admin.ops.performance.causes.${cause}`
  const translated = t(key)
  return translated === key ? cause : translated
}
</script>

<template>
  <div class="mc-impact">
    <div class="mc-impact-head">
      <div><h4>{{ t('admin.monitorCenter.slow.impact') }}</h4><p>{{ t('admin.monitorCenter.slow.impactHint') }}</p></div>
      <div class="mc-segmented" role="tablist" :aria-label="t('admin.monitorCenter.slow.impact')">
        <button
          v-for="item in dimensions"
          :key="item"
          type="button"
          role="tab"
          :aria-selected="dimension === item"
          :tabindex="dimension === item ? 0 : -1"
          :class="{ active: dimension === item }"
          @click="dimension = item"
        >
          {{ t(`admin.monitorCenter.slow.dimensions.${item}`) }}
          <span class="mc-dimension-count">{{ dimensionCounts[item] }}</span>
        </button>
      </div>
    </div>
    <div class="mc-table-wrap">
      <table class="mc-table">
        <thead><tr>
          <th
            v-for="column in columns"
            :key="column.key"
            :class="{ numeric: column.numeric }"
            :aria-sort="ariaSort(column.key)"
          >
            <button
              type="button"
              class="mc-sort-button"
              :class="{ numeric: column.numeric, active: sortKey === column.key }"
              :data-sort-key="column.key"
              :title="t('admin.monitorCenter.slow.sortBy', { column: column.label })"
              @click="toggleSort(column.key)"
            >
              <span>{{ column.label }}</span>
              <ArrowUp v-if="sortKey === column.key && sortDirection === 'asc'" aria-hidden="true" />
              <ArrowDown v-else-if="sortKey === column.key" aria-hidden="true" />
              <ArrowUpDown v-else aria-hidden="true" />
            </button>
          </th>
        </tr></thead>
        <tbody>
          <tr v-for="item in rows" :key="`${item.dimension}:${item.id}`">
            <td><strong :title="item.name || item.id">{{ item.name || `#${item.id}` }}</strong></td>
            <td class="numeric">{{ item.request_count }}</td>
            <td class="numeric" :class="item.slow_rate >= 30 ? 'mc-bad' : item.slow_rate >= 10 ? 'mc-warn' : ''">{{ formatPercent(item.slow_rate, 1) }}</td>
            <td class="numeric">{{ formatMs(item.e2e_p95_ms) }}</td>
            <td class="numeric">{{ formatMs(item.ttft_p95_ms) }}</td>
            <td class="numeric">{{ formatMs(item.queue_p95_ms) }}</td>
            <td>{{ causeLabel(item.main_cause) }}</td>
          </tr>
          <tr v-if="!rows.length"><td colspan="7"><div class="mc-empty">{{ t('common.noData') }}</div></td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.mc-impact { min-width: 0; border-top: 1px solid var(--mc-line); padding-top: 12px; }
.mc-impact-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; margin-bottom: 9px; }
.mc-impact h4 { margin: 0; font-size: 12px; font-weight: 700; }
.mc-impact p { margin: 3px 0 0; color: var(--mc-subtle); font-size: 9px; }
.mc-impact .mc-table-wrap { max-height: 420px; }
.mc-dimension-count { min-width: 16px; border-radius: 999px; padding: 1px 5px; color: var(--mc-subtle); background: var(--mc-soft-strong); font-size: 9px; font-variant-numeric: tabular-nums; }
.mc-segmented button.active .mc-dimension-count { color: var(--mc-blue); background: color-mix(in srgb, var(--mc-blue) 10%, var(--mc-panel)); }
.mc-sort-button { display: flex; width: 100%; align-items: center; gap: 5px; border: 0; padding: 0; color: inherit; background: transparent; font: inherit; white-space: nowrap; }
.mc-sort-button.numeric { justify-content: flex-end; }
.mc-sort-button:hover, .mc-sort-button.active { color: var(--mc-blue); }
.mc-sort-button svg { width: 12px; height: 12px; flex: none; opacity: .55; }
.mc-sort-button.active svg { opacity: 1; }
.mc-table td:first-child { max-width: 190px; overflow: hidden; color: var(--mc-text); text-overflow: ellipsis; white-space: nowrap; }
.mc-table td:last-child { color: var(--mc-text); }
.mc-table .mc-empty { min-height: 98px; }
@media (max-width: 760px) {
  .mc-impact-head { flex-direction: column; }
}
</style>
