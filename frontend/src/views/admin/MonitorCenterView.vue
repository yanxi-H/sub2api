<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { CalendarRange, RefreshCw } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { monitorCenterAPI, type MonitorCenterOpenAIHistoryResponse, type MonitorCenterOpenAIStatusResponse, type MonitorCenterProbeResponse, type MonitorCenterRangeData, type MonitorCenterRangeParams, type MonitorCenterThreeDayData } from '@/api/admin/monitorCenter'
import MonitorCockpit from '@/components/monitor-center/MonitorCockpit.vue'
import UpstreamStatusPanel from '@/components/monitor-center/UpstreamStatusPanel.vue'
import GatewayStatusPanel from '@/components/monitor-center/GatewayStatusPanel.vue'
import RealProbePanel from '@/components/monitor-center/RealProbePanel.vue'
import RequestLatencyChart from '@/components/monitor-center/RequestLatencyChart.vue'
import ConcurrencyLanesChart from '@/components/monitor-center/ConcurrencyLanesChart.vue'
import SlowRequestTimeline from '@/components/monitor-center/SlowRequestTimeline.vue'
import SlowRequestRanking from '@/components/monitor-center/SlowRequestRanking.vue'
import SlowImpactTable from '@/components/monitor-center/SlowImpactTable.vue'
import ProbeHistoryPanel from '@/components/monitor-center/ProbeHistoryPanel.vue'
import { formatDateTime, formatMs, formatPercent } from '@/components/monitor-center/monitorCenterUtils'
import '@/components/monitor-center/monitor-center.css'

type TimeRange = '1h' | '6h' | '24h' | 'custom'

const { t } = useI18n()
const selectedRange = ref<TimeRange>('1h')
const appliedRange = ref<TimeRange>('1h')
const customStart = ref('')
const customEnd = ref('')
const appliedCustomStart = ref('')
const appliedCustomEnd = ref('')
const customError = ref('')
const loading = ref(false)
const lastUpdated = ref<string | null>(null)
const loadWarning = ref('')
const rangeData = ref<MonitorCenterRangeData | null>(null)
const openAIStatus = ref<MonitorCenterOpenAIStatusResponse | null>(null)
const openAIRangeHistory = ref<MonitorCenterOpenAIHistoryResponse | null>(null)
const probeRange = ref<MonitorCenterProbeResponse | null>(null)
let requestController: AbortController | null = null
let requestSequence = 0
let activeRequest: Promise<void> | null = null

function toLocalInput(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

const now = new Date()
customEnd.value = toLocalInput(now)
customStart.value = toLocalInput(new Date(now.getTime() - 60 * 60 * 1000))
const maxDateTime = computed(() => toLocalInput(new Date()))

function buildRangeParams(): MonitorCenterRangeParams {
  if (appliedRange.value === 'custom') {
    return {
      start_time: new Date(appliedCustomStart.value).toISOString(),
      end_time: new Date(appliedCustomEnd.value).toISOString(),
    }
  }
  return { time_range: appliedRange.value }
}

function isCanceled(error: unknown): boolean {
  return !!error && typeof error === 'object' && (
    ('name' in error && (error as { name?: string }).name === 'AbortError')
    || ('code' in error && (error as { code?: string }).code === 'ERR_CANCELED')
  )
}

async function loadData(options: { force?: boolean } = {}): Promise<void> {
  if (activeRequest && !options.force) return activeRequest
  if (options.force) requestController?.abort()
  const sequence = ++requestSequence
  const controller = new AbortController()
  requestController = controller
  loading.value = true
  loadWarning.value = ''

  const execute = async () => {
    const rangeParams = buildRangeParams()
    const requests = [
      monitorCenterAPI.getRangeData(rangeParams, controller.signal),
      monitorCenterAPI.getOpenAIStatus(controller.signal),
      monitorCenterAPI.getOpenAIHistory(rangeParams, controller.signal),
      monitorCenterAPI.getProbe(rangeParams, controller.signal),
    ] as const
    const results = await Promise.allSettled(requests)
    if (sequence !== requestSequence) return

    let nestedFailures = 0
    let successfulRequests = 0
    if (results[0].status === 'fulfilled') {
      rangeData.value = { ...(rangeData.value ?? {}), ...results[0].value.data }
      nestedFailures += results[0].value.failure_count
      successfulRequests += results[0].value.success_count
    }
    if (results[1].status === 'fulfilled') openAIStatus.value = results[1].value
    if (results[2].status === 'fulfilled') openAIRangeHistory.value = results[2].value
    if (results[3].status === 'fulfilled') probeRange.value = results[3].value
    successfulRequests += results.slice(1, 4).filter((result) => result.status === 'fulfilled').length

    const topLevelFailures = results.filter((result) => result.status === 'rejected' && !isCanceled(result.reason)).length
    const failureCount = topLevelFailures + nestedFailures
    if (failureCount) loadWarning.value = t('admin.monitorCenter.loadPartial', { count: failureCount })
    if (successfulRequests > 0) lastUpdated.value = new Date().toISOString()
  }

  const promise = execute().finally(() => {
    if (sequence === requestSequence) {
      loading.value = false
      requestController = null
    }
    if (activeRequest === promise) activeRequest = null
  })
  activeRequest = promise
  return promise
}

function selectRange(range: TimeRange) {
  selectedRange.value = range
  customError.value = ''
  if (range === 'custom') return
  appliedRange.value = range
  void loadData({ force: true })
}

function applyCustomRange() {
  customError.value = ''
  if (!customStart.value || !customEnd.value) {
    customError.value = t('admin.monitorCenter.custom.required')
    return
  }
  const start = new Date(customStart.value)
  const end = new Date(customEnd.value)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    customError.value = t('admin.monitorCenter.custom.invalid')
    return
  }
  if (start >= end) {
    customError.value = t('admin.monitorCenter.custom.order')
    return
  }
  if (end.getTime() - start.getTime() > 30 * 24 * 60 * 60 * 1000) {
    customError.value = t('admin.monitorCenter.custom.tooLong')
    return
  }
  appliedCustomStart.value = customStart.value
  appliedCustomEnd.value = customEnd.value
  appliedRange.value = 'custom'
  void loadData({ force: true })
}

function refresh() {
  void loadData()
}

const { pause, resume } = useIntervalFn(() => {
  if (document.hidden) return
  void loadData()
}, 60_000, { immediate: false })

function handleVisibilityChange() {
  if (document.hidden) {
    pause()
  } else {
    void loadData()
    resume()
  }
}

onMounted(() => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  void loadData()
  if (!document.hidden) resume()
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  pause()
  requestSequence += 1
  requestController?.abort()
  requestController = null
})

const slowPrimaryCause = computed(() => rangeData.value?.performance?.causes?.[0])
const appliedRangeLabel = computed(() => t(`admin.monitorCenter.ranges.${appliedRange.value}`))
const historyData = computed<MonitorCenterThreeDayData>(() => ({
  openai: openAIRangeHistory.value ?? undefined,
  probe: probeRange.value ?? undefined,
  errors: rangeData.value?.errors,
  throughput: rangeData.value?.throughput,
}))
function causeLabel(cause?: string): string {
  if (!cause) return t('admin.monitorCenter.slow.noSlowRequests')
  const key = `admin.ops.performance.causes.${cause}`
  const translated = t(key)
  return translated === key ? cause : translated
}
</script>

<template>
  <AppLayout>
    <div class="monitor-center mc-stack">
      <header class="mc-toolbar mc-panel">
        <div class="mc-toolbar-title">
          <strong>{{ t('admin.monitorCenter.title') }}</strong>
          <span>{{ t('admin.monitorCenter.description') }}</span>
        </div>
        <div class="mc-toolbar-actions">
          <span class="mc-auto-refresh"><i />{{ t('admin.monitorCenter.autoRefresh') }}</span>
          <div class="mc-segmented" role="group" :aria-label="t('admin.monitorCenter.timeRange')">
            <button v-for="range in (['1h', '6h', '24h', 'custom'] as TimeRange[])" :key="range" type="button" :aria-pressed="selectedRange === range" :class="{ active: selectedRange === range }" @click="selectRange(range)">
              {{ t(`admin.monitorCenter.ranges.${range}`) }}
            </button>
          </div>
          <button type="button" class="mc-button mc-icon-button" :disabled="loading" :title="t('common.refresh')" @click="refresh">
            <RefreshCw :class="{ 'animate-spin': loading }" />
          </button>
        </div>
        <div v-if="selectedRange === 'custom'" class="mc-custom-range">
          <CalendarRange />
          <label><span>{{ t('admin.monitorCenter.custom.start') }}</span><input v-model="customStart" class="mc-input" type="datetime-local" :max="maxDateTime" /></label>
          <label><span>{{ t('admin.monitorCenter.custom.end') }}</span><input v-model="customEnd" class="mc-input" type="datetime-local" :max="maxDateTime" /></label>
          <button type="button" class="mc-button primary" :disabled="loading" @click="applyCustomRange">{{ t('admin.monitorCenter.custom.apply') }}</button>
          <span v-if="customError" class="mc-custom-error">{{ customError }}</span>
        </div>
        <div class="mc-toolbar-meta">{{ t('admin.monitorCenter.lastUpdated', { time: formatDateTime(lastUpdated) }) }}</div>
      </header>

      <div v-if="loadWarning" class="mc-notice">{{ loadWarning }} {{ t('admin.monitorCenter.keepPrevious') }}</div>

      <MonitorCockpit :overview="rangeData?.overview ?? null" :throughput="rangeData?.throughput ?? null" :loading="loading" />

      <section class="mc-section-grid">
        <UpstreamStatusPanel :status="openAIStatus" :history="openAIRangeHistory" :range-label="appliedRangeLabel" :loading="loading" />
        <GatewayStatusPanel :overview="rangeData?.overview ?? null" :errors="rangeData?.errors ?? null" :throughput="rangeData?.throughput ?? null" :loading="loading" />
        <RealProbePanel :probe="probeRange" :official-history="openAIRangeHistory" :loading="loading" />
      </section>

      <RequestLatencyChart :points="rangeData?.latency?.points ?? []" :overview="rangeData?.overview ?? null" :loading="loading" :range="appliedRange" />

      <ConcurrencyLanesChart :data="rangeData?.concurrency ?? null" :loading="loading" @refresh="loadData({ force: true })" />

      <section class="mc-panel mc-panel-pad mc-slow-section">
        <div class="mc-panel-head mc-slow-head">
          <div>
            <div class="mc-panel-title">{{ t('admin.monitorCenter.slow.title') }}</div>
            <div class="mc-panel-subtitle">{{ slowPrimaryCause ? t('admin.monitorCenter.slow.primaryCause', { cause: causeLabel(slowPrimaryCause.cause), share: formatPercent(slowPrimaryCause.share, 1) }) : t('admin.monitorCenter.slow.noSlowRequests') }}</div>
          </div>
          <div class="mc-slow-summary">
            <div><span>E2E P95</span><strong>{{ formatMs(rangeData?.performance?.summary.end_to_end.p95_ms) }}</strong></div>
            <div><span>TTFT P95</span><strong>{{ formatMs(rangeData?.performance?.summary.ttft.p95_ms) }}</strong></div>
            <div><span>{{ t('admin.monitorCenter.slow.slowRate') }}</span><strong>{{ formatPercent(rangeData?.performance?.summary.slow_rate) }}</strong></div>
          </div>
        </div>
        <div v-if="rangeData?.performance?.ingestion_health && (rangeData.performance.ingestion_health.dropped_count > 0 || rangeData.performance.ingestion_health.write_failed_count > 0)" class="mc-notice mc-ingestion">
          {{ t('admin.monitorCenter.slow.ingestionWarning', { dropped: rangeData.performance.ingestion_health.dropped_count, failed: rangeData.performance.ingestion_health.write_failed_count }) }}
        </div>
        <SlowRequestTimeline :data="rangeData?.performance ?? null" :loading="loading" :range="appliedRange" />
        <div class="mc-slow-layout">
          <SlowRequestRanking :causes="rangeData?.performance?.causes ?? []" />
          <SlowImpactTable :impacts="rangeData?.performance?.impacts ?? []" />
        </div>
      </section>

      <ProbeHistoryPanel :data="historyData" :range-label="appliedRangeLabel" :loading="loading" />
    </div>
  </AppLayout>
</template>

<style scoped>
.mc-toolbar { position: sticky; top: 78px; z-index: 20; display: grid; grid-template-columns: minmax(220px, 1fr) auto; align-items: center; gap: 9px 16px; padding: 10px 12px; background: color-mix(in srgb, var(--mc-panel) 92%, transparent); backdrop-filter: blur(18px) saturate(140%); }
.mc-toolbar-title { min-width: 0; }
.mc-toolbar-title strong { display: block; font-size: 14px; }
.mc-toolbar-title span { display: block; overflow: hidden; margin-top: 2px; color: var(--mc-subtle); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.mc-toolbar-actions { display: flex; align-items: center; justify-content: flex-end; gap: 7px; }
.mc-auto-refresh { display: inline-flex; align-items: center; gap: 6px; color: var(--mc-muted); font-size: 10px; white-space: nowrap; }
.mc-auto-refresh i { width: 6px; height: 6px; border-radius: 50%; background: var(--mc-green); box-shadow: 0 0 0 3px color-mix(in srgb, var(--mc-green) 12%, transparent); }
.mc-custom-range { grid-column: 1 / -1; display: flex; align-items: end; justify-content: flex-end; gap: 8px; border-top: 1px solid var(--mc-line); padding-top: 9px; }
.mc-custom-range > svg { width: 16px; height: 16px; align-self: center; color: var(--mc-subtle); }
.mc-custom-range label span { display: block; margin-bottom: 3px; color: var(--mc-subtle); font-size: 9px; }
.mc-custom-error { align-self: center; color: var(--mc-red); font-size: 10px; }
.mc-toolbar-meta { position: absolute; right: 14px; bottom: -17px; color: var(--mc-subtle); font-size: 8px; }
.mc-slow-head { align-items: center; }
.mc-slow-summary { display: grid; grid-template-columns: repeat(3, auto); gap: 22px; text-align: right; }
.mc-slow-summary span { display: block; color: var(--mc-subtle); font-size: 9px; font-weight: 650; }
.mc-slow-summary strong { display: block; margin-top: 3px; font-size: 13px; font-variant-numeric: tabular-nums; }
.mc-ingestion { margin-bottom: 10px; }
.mc-slow-layout { display: grid; grid-template-columns: minmax(260px, .78fr) minmax(0, 1.22fr); gap: 14px; margin-top: 11px; }
@media (max-width: 1000px) {
  .mc-toolbar { top: 70px; grid-template-columns: 1fr; }
  .mc-toolbar-actions { justify-content: flex-start; flex-wrap: wrap; }
  .mc-slow-layout { grid-template-columns: 1fr; }
}
@media (max-width: 760px) {
  .mc-toolbar { position: relative; top: auto; }
  .mc-auto-refresh { display: none; }
  .mc-custom-range { align-items: stretch; flex-direction: column; }
  .mc-custom-range > svg { display: none; }
  .mc-slow-head { align-items: stretch; }
  .mc-slow-summary { grid-template-columns: repeat(3, 1fr); gap: 8px; text-align: left; }
}
</style>
