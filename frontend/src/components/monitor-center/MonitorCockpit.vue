<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Activity,
  Cpu,
  Database,
  MemoryStick,
  RadioTower,
  ServerCog,
} from '@lucide/vue'
import {
  ArcElement,
  CategoryScale,
  Chart as ChartJS,
  DoughnutController,
  Filler,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
} from 'chart.js'
import { Doughnut, Line } from 'vue-chartjs'
import type { OpsDashboardOverview, OpsThroughputTrendResponse } from '@/api/admin/monitorCenter'
import { chartPalette, formatCompactNumber, formatNumber, formatPercent } from './monitorCenterUtils'

ChartJS.register(ArcElement, DoughnutController, CategoryScale, LinearScale, PointElement, LineElement, Filler, Tooltip)

const props = defineProps<{
  overview: OpsDashboardOverview | null
  throughput: OpsThroughputTrendResponse | null
  loading: boolean
}>()

const { t } = useI18n()
const score = computed(() => Math.max(0, Math.min(100, Math.round(props.overview?.health_score ?? 0))))
const scoreTone = computed(() => score.value >= 85 ? 'good' : score.value >= 65 ? 'warn' : 'bad')
const scoreLabel = computed(() => t(`admin.monitorCenter.health.${scoreTone.value}`))

const gaugeData = computed(() => ({
  datasets: [{
    data: [score.value, Math.max(0, 100 - score.value)],
    backgroundColor: [
      scoreTone.value === 'good' ? '#4f9d73' : scoreTone.value === 'warn' ? '#c98524' : '#c84d49',
      document.documentElement.classList.contains('dark') ? '#343437' : '#e6e6e9',
    ],
    borderWidth: 0,
    circumference: 300,
    rotation: 210,
  }],
}))

const gaugeOptions = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '82%',
  plugins: { tooltip: { enabled: false } },
}

const sparkData = computed(() => ({
  labels: (props.throughput?.points ?? []).map((point) => point.bucket_start),
  datasets: [{
    label: 'TPS',
    data: (props.throughput?.points ?? []).map((point) => point.tps),
    borderColor: '#78a8df',
    backgroundColor: 'transparent',
    borderWidth: 2,
    pointRadius: 0,
    pointHitRadius: 8,
    tension: .3,
    spanGaps: false,
  }],
}))

const sparkOptions = computed(() => {
  const palette = chartPalette()
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: palette.tooltipBg,
        borderColor: palette.tooltipBorder,
        borderWidth: 1,
        titleColor: palette.title,
        bodyColor: palette.text,
        callbacks: { label: (context: any) => `TPS ${formatNumber(context.parsed.y, 1)}` },
      },
    },
    scales: { x: { display: false }, y: { display: false, beginAtZero: true } },
  }
})

const system = computed(() => props.overview?.system_metrics)
const jobsHealthy = computed(() => {
  const jobs = props.overview?.job_heartbeats ?? []
  if (!jobs.length) return null
  return !jobs.some((job) => {
    if (!job.last_error_at) return false
    if (!job.last_success_at) return true
    return new Date(job.last_error_at).getTime() > new Date(job.last_success_at).getTime()
  })
})

const resources = computed(() => [
  {
    key: 'cpu', icon: Cpu, label: t('admin.monitorCenter.resources.cpu'),
    value: formatPercent(system.value?.cpu_usage_percent, 1),
    detail: t('admin.monitorCenter.resources.cpuHint'), tone: (system.value?.cpu_usage_percent ?? 0) >= 80 ? 'warn' : 'good',
  },
  {
    key: 'memory', icon: MemoryStick, label: t('admin.monitorCenter.resources.memory'),
    value: formatPercent(system.value?.memory_usage_percent, 1),
    detail: `${formatNumber(system.value?.memory_used_mb, 0)} / ${formatNumber(system.value?.memory_total_mb, 0)} MB`,
    tone: (system.value?.memory_usage_percent ?? 0) >= 80 ? 'warn' : 'good',
  },
  {
    key: 'database', icon: Database, label: t('admin.monitorCenter.resources.database'),
    value: system.value?.db_ok == null ? '-' : t(`admin.monitorCenter.common.${system.value.db_ok ? 'healthy' : 'abnormal'}`),
    detail: t('admin.monitorCenter.resources.databaseDetail', { active: system.value?.db_conn_active ?? '-', idle: system.value?.db_conn_idle ?? '-' }),
    tone: system.value?.db_ok === false ? 'bad' : system.value?.db_ok === true ? 'good' : 'unknown',
  },
  {
    key: 'redis', icon: RadioTower, label: t('admin.monitorCenter.resources.redis'),
    value: system.value?.redis_ok == null ? '-' : t(`admin.monitorCenter.common.${system.value.redis_ok ? 'healthy' : 'abnormal'}`),
    detail: t('admin.monitorCenter.resources.redisDetail', { total: system.value?.redis_conn_total ?? '-', idle: system.value?.redis_conn_idle ?? '-' }),
    tone: system.value?.redis_ok === false ? 'bad' : system.value?.redis_ok === true ? 'good' : 'unknown',
  },
  {
    key: 'goroutines', icon: Activity, label: t('admin.monitorCenter.resources.goroutines'),
    value: formatNumber(system.value?.goroutine_count, 0),
    detail: t('admin.monitorCenter.resources.goroutineDetail', { queue: system.value?.concurrency_queue_depth ?? '-' }),
    tone: (system.value?.goroutine_count ?? 0) >= 8000 ? 'warn' : 'good',
  },
  {
    key: 'jobs', icon: ServerCog, label: t('admin.monitorCenter.resources.jobs'),
    value: jobsHealthy.value == null ? '-' : t(`admin.monitorCenter.common.${jobsHealthy.value ? 'healthy' : 'abnormal'}`),
    detail: t('admin.monitorCenter.resources.jobsDetail', { count: props.overview?.job_heartbeats?.length ?? 0 }),
    tone: jobsHealthy.value === false ? 'bad' : jobsHealthy.value === true ? 'good' : 'unknown',
  },
])

const kpis = computed(() => [
  { key: 'requests', label: t('admin.monitorCenter.cockpit.requests'), value: formatCompactNumber(props.overview?.request_count_total), detail: t('admin.monitorCenter.cockpit.tokensDetail', { value: formatCompactNumber(props.overview?.token_consumed) }), tone: '' },
  { key: 'sla', label: 'SLA', value: formatPercent(props.overview?.sla, 3), detail: t('admin.monitorCenter.cockpit.errorsDetail', { value: props.overview?.error_count_sla ?? '-' }), tone: (props.overview?.sla ?? 100) < 99 ? 'bad' : 'good' },
  { key: 'requestError', label: t('admin.monitorCenter.cockpit.requestError'), value: formatPercent(props.overview?.error_rate), detail: t('admin.monitorCenter.cockpit.businessLimitsDetail', { value: props.overview?.business_limited_count ?? '-' }), tone: (props.overview?.error_rate ?? 0) >= 3 ? 'bad' : (props.overview?.error_rate ?? 0) >= 1 ? 'warn' : 'good' },
  { key: 'e2e', label: t('admin.monitorCenter.cockpit.e2eP99'), value: formatNumber(props.overview?.duration?.p99_ms, 0), suffix: 'ms', detail: `P95 ${formatNumber(props.overview?.duration?.p95_ms, 0)} · Avg ${formatNumber(props.overview?.duration?.avg_ms, 0)}`, tone: '' },
  { key: 'ttft', label: t('admin.monitorCenter.cockpit.ttftP99'), value: formatNumber(props.overview?.ttft?.p99_ms, 0), suffix: 'ms', detail: `P95 ${formatNumber(props.overview?.ttft?.p95_ms, 0)} · Avg ${formatNumber(props.overview?.ttft?.avg_ms, 0)}`, tone: '' },
  { key: 'upstreamError', label: t('admin.monitorCenter.cockpit.upstreamError'), value: formatPercent(props.overview?.upstream_error_rate), detail: t('admin.monitorCenter.cockpit.upstreamExclusions'), tone: (props.overview?.upstream_error_rate ?? 0) >= 3 ? 'bad' : (props.overview?.upstream_error_rate ?? 0) >= 1 ? 'warn' : 'good' },
])
</script>

<template>
  <section class="mc-cockpit mc-panel" :class="{ 'mc-loading': loading && !overview }">
    <div class="mc-health">
      <div class="mc-gauge">
        <Doughnut :data="gaugeData" :options="gaugeOptions" />
        <div class="mc-gauge-copy">
          <strong :class="`mc-${scoreTone}`">{{ score }}</strong>
          <span>{{ t('admin.monitorCenter.cockpit.health') }}</span>
        </div>
      </div>
      <div>
        <div class="mc-overline">{{ t('admin.monitorCenter.cockpit.overallHealth') }}</div>
        <div class="mc-health-label">{{ scoreLabel }}</div>
        <div class="mc-panel-subtitle">{{ t('admin.monitorCenter.cockpit.healthHint') }}</div>
      </div>
    </div>

    <div class="mc-realtime">
      <div class="mc-overline">{{ t('admin.monitorCenter.cockpit.realtime') }}</div>
      <div class="mc-realtime-grid">
        <div><span>{{ t('admin.monitorCenter.cockpit.currentQps') }}</span><strong>{{ formatNumber(overview?.qps.current, 2) }}</strong></div>
        <div><span>{{ t('admin.monitorCenter.cockpit.currentTps') }}</span><strong>{{ formatNumber(overview?.tps.current, 1) }}</strong></div>
        <div><span>{{ t('admin.monitorCenter.cockpit.peakTps') }}</span><strong>{{ formatNumber(overview?.tps.peak, 1) }}</strong></div>
        <div><span>{{ t('admin.monitorCenter.cockpit.averageTps') }}</span><strong>{{ formatNumber(overview?.tps.avg, 1) }}</strong></div>
      </div>
      <div class="mc-spark"><Line :data="sparkData" :options="sparkOptions" /></div>
    </div>

    <div class="mc-kpis">
      <div v-for="item in kpis" :key="item.key" class="mc-kpi" :class="item.tone ? `tone-${item.tone}` : ''">
        <span>{{ item.label }}</span>
        <strong class="mc-metric">{{ item.value }} <small v-if="item.suffix">{{ item.suffix }}</small></strong>
        <small>{{ item.detail }}</small>
      </div>
    </div>
  </section>

  <section class="mc-resources">
    <article v-for="item in resources" :key="item.key" class="mc-resource">
      <div class="mc-resource-head"><component :is="item.icon" /><span>{{ item.label }}</span></div>
      <strong class="mc-metric" :class="`mc-${item.tone}`">{{ item.value }}</strong>
      <small>{{ item.detail }}</small>
    </article>
  </section>
</template>

<style scoped>
.mc-cockpit { display: grid; grid-template-columns: 250px minmax(310px, .95fr) minmax(0, 1.7fr); gap: 10px; padding: 11px; }
.mc-health, .mc-realtime { min-width: 0; border-radius: 7px; padding: 14px; background: var(--mc-soft); }
.mc-health { display: grid; grid-template-columns: 104px 1fr; align-items: center; gap: 12px; }
.mc-gauge { position: relative; width: 100px; height: 100px; }
.mc-gauge-copy { position: absolute; inset: 0; display: grid; place-content: center; text-align: center; pointer-events: none; }
.mc-gauge-copy strong { font-size: 27px; line-height: 1; font-weight: 750; }
.mc-gauge-copy span { margin-top: 4px; color: var(--mc-subtle); font-size: 9px; }
.mc-overline { color: var(--mc-subtle); font-size: 9px; font-weight: 700; text-transform: uppercase; }
.mc-health-label { margin-top: 6px; font-size: 19px; font-weight: 720; }
.mc-realtime-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 14px; margin-top: 10px; }
.mc-realtime-grid span { display: block; color: var(--mc-subtle); font-size: 9px; }
.mc-realtime-grid strong { display: block; margin-top: 2px; color: var(--mc-text); font-size: 19px; line-height: 1.15; font-weight: 690; font-variant-numeric: tabular-nums; }
.mc-spark { height: 40px; margin-top: 4px; }
.mc-kpis { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 7px; }
.mc-kpi { min-width: 0; min-height: 88px; border-radius: 7px; padding: 11px; background: var(--mc-soft); }
.mc-kpi > span { display: block; color: var(--mc-subtle); font-size: 9px; font-weight: 700; text-transform: uppercase; }
.mc-kpi > strong { display: block; overflow: hidden; margin-top: 6px; color: var(--mc-text); font-size: 20px; line-height: 1.12; font-weight: 710; text-overflow: ellipsis; white-space: nowrap; }
.mc-kpi > strong small { font-size: 10px; font-weight: 650; }
.mc-kpi > small { display: block; overflow: hidden; margin-top: 6px; color: var(--mc-subtle); font-size: 9px; line-height: 1.3; text-overflow: ellipsis; white-space: nowrap; }
.mc-kpi.tone-good > strong { color: var(--mc-green); }
.mc-kpi.tone-warn > strong { color: var(--mc-orange); }
.mc-kpi.tone-bad > strong { color: var(--mc-red); }
.mc-resources { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 7px; }
.mc-resource { min-width: 0; border: 1px solid color-mix(in srgb, var(--mc-line) 68%, transparent); border-radius: 7px; padding: 10px 11px; background: var(--mc-panel); }
.mc-resource-head { display: flex; align-items: center; gap: 6px; color: var(--mc-subtle); font-size: 9px; font-weight: 700; text-transform: uppercase; }
.mc-resource-head svg { width: 13px; height: 13px; }
.mc-resource > strong { display: block; overflow: hidden; margin-top: 5px; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.mc-resource > small { display: block; overflow: hidden; margin-top: 3px; color: var(--mc-subtle); font-size: 9px; line-height: 1.3; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 1280px) {
  .mc-cockpit { grid-template-columns: 1fr 1fr; }
  .mc-kpis { grid-column: 1 / -1; }
  .mc-resources { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 760px) {
  .mc-cockpit { grid-template-columns: 1fr; }
  .mc-kpis { grid-column: auto; grid-template-columns: 1fr 1fr; }
  .mc-resources { grid-template-columns: 1fr 1fr; }
}
@media (max-width: 430px) {
  .mc-kpis, .mc-resources { grid-template-columns: 1fr; }
}
</style>
