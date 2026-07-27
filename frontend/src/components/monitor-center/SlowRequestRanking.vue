<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpsSlowCauseSummary } from '@/api/admin/ops'
import { formatMs, formatPercent } from './monitorCenterUtils'

const props = defineProps<{ causes: OpsSlowCauseSummary[] }>()
const { t } = useI18n()
const maxCount = computed(() => Math.max(...props.causes.map((item) => item.count), 1))
function causeLabel(cause: string): string {
  const key = `admin.ops.performance.causes.${cause}`
  const translated = t(key)
  return translated === key ? cause : translated
}
</script>

<template>
  <div class="mc-ranking">
    <h4>{{ t('admin.monitorCenter.slow.ranking') }}</h4>
    <div v-if="causes.length" class="mc-rank-list">
      <div v-for="item in causes" :key="item.cause" class="mc-rank-row">
        <div><strong>{{ causeLabel(item.cause) }}</strong><span>{{ item.count }} · {{ formatPercent(item.share, 1) }}</span></div>
        <div class="mc-progress"><i :style="{ width: `${Math.max(3, item.count / maxCount * 100)}%` }" /></div>
        <small>E2E P95 {{ formatMs(item.e2e_p95_ms) }} · TTFT P95 {{ formatMs(item.ttft_p95_ms) }} · {{ t('admin.monitorCenter.slow.queueP95') }} {{ formatMs(item.queue_p95_ms) }}</small>
      </div>
    </div>
    <div v-else class="mc-empty">{{ t('admin.monitorCenter.slow.noSlowRequests') }}</div>
  </div>
</template>

<style scoped>
.mc-ranking { min-width: 0; border-top: 1px solid var(--mc-line); padding-top: 12px; }
.mc-ranking h4 { margin: 0 0 11px; font-size: 12px; font-weight: 700; }
.mc-rank-list { display: grid; gap: 12px; }
.mc-rank-row > div:first-child { display: flex; justify-content: space-between; gap: 10px; font-size: 10px; }
.mc-rank-row > div:first-child strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mc-rank-row > div:first-child span { flex: none; color: var(--mc-muted); font-variant-numeric: tabular-nums; }
.mc-progress { height: 5px; overflow: hidden; margin-top: 6px; border-radius: 4px; background: var(--mc-soft-strong); }
.mc-progress i { display: block; height: 100%; border-radius: inherit; background: var(--mc-orange); }
.mc-rank-row small { display: block; margin-top: 5px; color: var(--mc-subtle); font-size: 9px; line-height: 1.45; }
.mc-ranking .mc-empty { min-height: 160px; }
</style>
