<template>
  <section class="model-recommendations card overflow-hidden" aria-labelledby="model-recommendations-title">
    <header class="flex min-h-16 items-center justify-between gap-4 border-b border-gray-100 px-4 py-3 dark:border-dark-700 sm:px-6">
      <div class="flex min-w-0 items-center gap-3">
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-100 dark:bg-primary-900/30">
          <Icon name="sparkles" size="sm" class="text-primary-600 dark:text-primary-400" />
        </div>
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-2">
            <h2 id="model-recommendations-title" class="truncate text-base font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.modelRecommendations.title') }}
            </h2>
            <span v-if="hasData" class="hidden shrink-0 rounded-full bg-primary-50 px-2 py-0.5 text-[10px] font-medium text-primary-700 sm:inline-flex dark:bg-primary-900/30 dark:text-primary-300">
              {{ t('dashboard.modelRecommendations.snapshot') }}
            </span>
          </div>
          <p v-if="formattedUpdatedAt" class="truncate text-xs text-gray-500 dark:text-gray-400">
            {{ t('dashboard.modelRecommendations.updatedAt', { time: formattedUpdatedAt }) }}
          </p>
        </div>
      </div>
      <button
        type="button"
        class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-gray-500 transition-colors duration-150 hover:bg-gray-100 hover:text-primary-600 active:scale-95 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-400"
        :aria-label="t('dashboard.modelRecommendations.refresh')"
        :title="t('dashboard.modelRecommendations.refresh')"
        :disabled="loading"
        data-model-recommendations-refresh
        @click="$emit('refresh')"
      >
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
      </button>
    </header>

    <div v-if="loading && !hasData" class="flex items-center justify-center py-10">
      <LoadingSpinner size="md" />
    </div>
    <div v-else-if="!hasData" class="px-5 py-7 text-sm text-gray-500 dark:text-gray-400">
      {{ t('dashboard.modelRecommendations.unavailable') }}
    </div>
    <div v-else>
      <section v-if="summaryCards.length" class="border-b border-gray-100 bg-gray-50/60 px-4 py-4 dark:border-dark-700 dark:bg-dark-900/20 sm:px-6">
        <div class="mb-3 flex flex-wrap items-end justify-between gap-2">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.modelRecommendations.summaryTitle') }}
            </h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('dashboard.modelRecommendations.summaryDescription') }}
            </p>
          </div>
          <span class="hidden text-[11px] text-gray-400 sm:inline dark:text-gray-500">
            {{ t('dashboard.modelRecommendations.summaryHint') }}
          </span>
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <article
            v-for="summary in summaryCards"
            :key="summary.key"
            class="summary-card min-w-0 rounded-xl border border-gray-200/80 bg-white px-3.5 py-3 dark:border-dark-700 dark:bg-dark-800/70"
          >
            <div class="flex items-start gap-3">
              <div class="summary-icon" :data-tone="summary.tone">
                <Icon :name="summary.icon" size="sm" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex items-center justify-between gap-2">
                  <span class="truncate text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ summary.title }}</span>
                  <span class="shrink-0 rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ effortLabel(summary.item.effort) }}
                  </span>
                </div>
                <p class="mt-1 truncate font-mono text-sm font-semibold text-gray-900 dark:text-white" :title="modelVariantName(summary.item.model, summary.item.effort)">
                  {{ modelVariantName(summary.item.model, summary.item.effort) }}
                </p>
                <div class="mt-2 flex items-baseline gap-1.5">
                  <strong class="font-mono text-lg leading-none text-gray-900 dark:text-white">{{ summary.value }}</strong>
                  <span class="text-[11px] text-gray-500 dark:text-gray-400">{{ summary.valueLabel }}</span>
                </div>
              </div>
            </div>
            <div class="summary-meta mt-3 border-t border-gray-100 pt-2 dark:border-dark-700">
              <span class="summary-meta-item summary-meta-price" :title="t('dashboard.modelRecommendations.price')">
                <Icon name="dollar" size="xs" />
                <strong>{{ formatPrice(summary.item.average_cost_usd) }}</strong>
              </span>
              <span class="summary-meta-item summary-meta-time" :title="t('dashboard.modelRecommendations.time')">
                <Icon name="clock" size="xs" />
                <strong>{{ formatDuration(summary.item.average_duration_minutes) }}</strong>
              </span>
            </div>
          </article>
        </div>
      </section>

      <section v-if="stationGroups.length" class="recommendation-section border-b border-gray-100 px-4 py-5 dark:border-dark-700 sm:px-6">
        <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
          <div>
            <p class="text-[10px] font-semibold uppercase tracking-[0.16em] text-primary-600 dark:text-primary-400">01 / {{ t('dashboard.modelRecommendations.station') }}</p>
            <h3 class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.modelRecommendations.station') }}
            </h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('dashboard.modelRecommendations.stationDescription') }}
            </p>
          </div>
          <span class="section-count">
            <Icon name="grid" size="xs" />
            {{ stationGroups.length }} {{ t('dashboard.modelRecommendations.scenes') }}
          </span>
        </div>

        <div class="grid grid-cols-1 gap-3 md:grid-cols-2 2xl:grid-cols-4">
          <article v-for="(group, index) in stationGroups" :key="group.key" class="station-lane min-w-0 rounded-xl border border-gray-200/80 bg-gray-50/50 p-3.5 dark:border-dark-700 dark:bg-dark-900/20">
            <header class="flex min-w-0 items-center gap-2.5">
              <span class="station-index" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</span>
              <div class="min-w-0">
                <h4 class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="stationTitle(group)">
                  {{ stationTitle(group) }}
                </h4>
                <p class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.modelRecommendations.stationLaneHint') }}
                </p>
              </div>
            </header>

            <div v-if="stationPrimary(group)" class="station-primary mt-3 rounded-lg border border-primary-100 bg-white p-3 dark:border-primary-900/50 dark:bg-dark-800/70">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <span class="text-[10px] font-medium uppercase tracking-wide text-primary-700 dark:text-primary-300">
                    {{ t('dashboard.modelRecommendations.preferred') }}
                  </span>
                  <h5 class="mt-1.5 truncate font-mono text-sm font-semibold text-gray-900 dark:text-white" :title="modelVariantName(stationPrimary(group)!.model, stationPrimary(group)!.effort)">
                    {{ modelVariantName(stationPrimary(group)!.model, stationPrimary(group)!.effort) }}
                  </h5>
                </div>
                <div class="shrink-0 text-right">
                  <span class="block text-[9px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
                    {{ t('dashboard.modelRecommendations.reasoningStrength') }}
                  </span>
                  <span class="effort-pill effort-pill-strong mt-1">{{ effortLabel(stationPrimary(group)!.effort) }}</span>
                </div>
              </div>
              <div class="station-score mt-3 border-t border-primary-50 pt-3 dark:border-primary-900/40">
                <div>
                  <span class="block text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
                    {{ t('dashboard.modelRecommendations.iqScore') }}
                  </span>
                  <div class="mt-0.5 flex items-baseline gap-1.5">
                    <strong class="font-mono text-2xl leading-none text-gray-900 dark:text-white">{{ formatIQ(stationPrimary(group)!.iq) }}</strong>
                    <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">IQ</span>
                  </div>
                </div>
                <span class="recommendation-callout">
                  <Icon name="star" size="xs" />
                  {{ t('dashboard.modelRecommendations.primary') }}
                </span>
              </div>
              <dl class="station-metric-grid mt-3 grid-cols-2 border-t border-gray-100 pt-3 text-[11px] dark:border-dark-700">
                <div class="station-metric-cell station-metric-price">
                  <dt :aria-label="t('dashboard.modelRecommendations.price')"><Icon name="dollar" size="xs" /></dt>
                  <dd>{{ formatPrice(stationPrimary(group)!.average_cost_usd) }}</dd>
                </div>
                <div class="station-metric-cell station-metric-time">
                  <dt :aria-label="t('dashboard.modelRecommendations.time')"><Icon name="clock" size="xs" /></dt>
                  <dd>{{ formatDuration(stationPrimary(group)!.average_duration_minutes) }}</dd>
                </div>
              </dl>
            </div>

            <div v-if="group.items.length > 1" class="station-alternatives mt-3 border-t border-gray-200/80 pt-2.5 dark:border-dark-700">
              <div class="station-alternatives-heading">
                <span class="station-alternatives-title">
                  <Icon name="grid" size="xs" />
                  {{ t('dashboard.modelRecommendations.alternatives') }}
                </span>
                <span class="station-alternatives-count">{{ group.items.length - 1 }}</span>
              </div>
              <div
                v-for="(item, alternativeIndex) in stationAlternatives(group)"
                :key="stationItemKey(group.key, item)"
                class="station-alternative flex min-w-0 items-center gap-2 py-2"
              >
                <span class="station-alt-index">{{ String(alternativeIndex + 2).padStart(2, '0') }}</span>
                <div class="min-w-0 flex-1">
                  <p class="truncate font-mono text-xs font-medium text-gray-800 dark:text-gray-100" :title="modelVariantName(item.model, item.effort)">{{ modelVariantName(item.model, item.effort) }}</p>
                  <p class="station-alternative-meta mt-0.5">
                    <span>IQ {{ formatIQ(item.iq) }}</span>
                    <span :title="t('dashboard.modelRecommendations.time')"><Icon name="clock" size="xs" />{{ formatDuration(item.average_duration_minutes) }}</span>
                  </p>
                </div>
                <div class="shrink-0 text-right">
                  <span class="effort-pill">{{ effortLabel(item.effort) }}</span>
                  <p class="station-alternative-price mt-1" :title="t('dashboard.modelRecommendations.price')"><Icon name="dollar" size="xs" />{{ formatPrice(item.average_cost_usd) }}</p>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section v-if="intelligenceGroups.length" class="recommendation-section px-4 py-5 dark:border-dark-700 sm:px-6">
        <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
          <div>
            <p class="text-[10px] font-semibold uppercase tracking-[0.16em] text-primary-600 dark:text-primary-400">02 / {{ t('dashboard.modelRecommendations.intelligence') }}</p>
            <h3 class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.modelRecommendations.intelligence') }}
            </h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('dashboard.modelRecommendations.intelligenceDescription') }}
            </p>
          </div>
          <button
            type="button"
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-gray-500 transition-colors duration-150 hover:bg-gray-100 hover:text-primary-600 active:scale-95 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-400"
            :aria-label="t('dashboard.modelRecommendations.refresh')"
            :title="t('dashboard.modelRecommendations.refresh')"
            :disabled="loading"
            data-intelligence-recommendations-refresh
            @click="$emit('refresh')"
          >
            <Icon name="refresh" size="sm" />
          </button>
        </div>

        <div class="space-y-3">
          <section
            v-for="group in intelligenceGroups"
            :key="group.model"
            class="intelligence-group"
          >
            <header class="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div class="flex min-w-0 items-center gap-2">
                <span class="model-mark" aria-hidden="true">{{ modelInitial(group.model) }}</span>
                <h4 class="truncate font-mono text-sm font-semibold text-gray-900 dark:text-white" :title="modelDisplayName(group.model)">{{ modelDisplayName(group.model) }}</h4>
              </div>
              <span class="text-[11px] text-gray-400 dark:text-gray-500">{{ group.items.length }} {{ t('dashboard.modelRecommendations.levels') }}</span>
            </header>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-5 2xl:grid-cols-6">
              <article
                v-for="item in group.items"
                :key="intelligenceItemKey(item)"
                class="summary-card intelligence-card min-w-0 rounded-xl border bg-white px-3 py-2.5 dark:bg-dark-800/60"
                :class="{ 'intelligence-card-best': isBest(group, item) }"
                :data-effort="item.effort"
                :style="{ '--reasoning-color': reasoningColor(item.effort), '--model-tier-color': modelTierColor(group.model) }"
              >
                <div class="flex items-start gap-3">
                  <div
                    class="reasoning-mark"
                    :data-effort="item.effort.trim().toLowerCase()"
                    :title="`${t('dashboard.modelRecommendations.reasoningStrength')}: ${effortLabel(item.effort)}`"
                    :aria-label="`${t('dashboard.modelRecommendations.reasoningStrength')}: ${effortLabel(item.effort)}`"
                  >
                    <ModelIcon :model="item.model" size="17px" :color="reasoningColor(item.effort)" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">{{ effortLabel(item.effort) }}</span>
                      <span class="shrink-0 text-[10px] font-mono text-gray-400 dark:text-gray-500">{{ intelligenceLevelIndex(group, item) }} / {{ group.items.length }}</span>
                    </div>
                    <p class="intelligence-model-name mt-1 min-h-5 break-words font-mono text-sm font-semibold leading-5" :title="modelVariantName(group.model, item.effort)">
                      {{ modelVariantName(group.model, item.effort) }}
                    </p>
                    <div class="mt-1.5 flex items-baseline gap-1.5">
                      <strong class="font-mono text-2xl leading-none text-gray-900 dark:text-white">{{ formatIQ(item.iq) }}</strong>
                      <span class="text-[11px] text-gray-500 dark:text-gray-400">IQ</span>
                    </div>
                  </div>
                </div>

                <div class="summary-meta mt-2.5 border-t border-gray-100 pt-2 dark:border-dark-700">
                  <span class="summary-meta-item summary-meta-price" :title="t('dashboard.modelRecommendations.price')">
                    <Icon name="dollar" size="xs" />
                    <strong>{{ formatPrice(item.average_cost_usd) }}</strong>
                  </span>
                  <span class="summary-meta-item summary-meta-time" :title="t('dashboard.modelRecommendations.time')">
                    <Icon name="clock" size="xs" />
                    <strong>{{ formatDuration(item.average_duration_minutes) }}</strong>
                  </span>
                  <span
                    v-if="isBest(group, item)"
                    class="best-stamp ml-auto flex shrink-0 items-center justify-center"
                    :title="t('dashboard.modelRecommendations.best')"
                    :aria-label="t('dashboard.modelRecommendations.best')"
                    :data-best-combination="intelligenceItemKey(item)"
                  >
                    <Icon name="star" size="xs" :stroke-width="2" />
                  </span>
                </div>
              </article>
            </div>
          </section>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import type {
  CodexRadarDashboardRecommendations,
  CodexRadarIntelligenceMetric,
  CodexRadarStationRecommendation,
  CodexRadarStationRecommendationSet
} from '@/api/usage'
import { formatDateTimeToMinute } from '@/utils/format'

interface IntelligenceGroup {
  model: string
  items: CodexRadarIntelligenceMetric[]
  bestKey: string | null
}

type SummaryTone = 'teal' | 'amber' | 'sky'
type SummaryIcon = 'brain' | 'star' | 'bolt'
interface SummaryCard {
  key: 'strongest' | 'balanced' | 'fastest'
  title: string
  item: CodexRadarIntelligenceMetric
  value: string
  valueLabel: string
  tone: SummaryTone
  icon: SummaryIcon
}

const props = withDefaults(defineProps<{
  data: CodexRadarDashboardRecommendations | null
  loading?: boolean
}>(), {
  loading: false
})

defineEmits<{
  refresh: []
}>()

const { t } = useI18n()

const effortOrder: Record<string, number> = {
  ultra: 0,
  max: 1,
  xhigh: 2,
  high: 3,
  medium: 4,
  low: 5
}

const modelOrder: Record<string, number> = {
  'gpt-5.6-sol': 0,
  'gpt-5.6-terra': 1,
  'gpt-5.6-luna': 2,
  'gpt-5.5': 3,
  'deepseek-v4-flash': 4
}

const modelDisplayNames: Record<string, string> = {
  'gpt-5.6-sol': '5.6 Sol',
  'gpt-5.6-terra': '5.6 Terra',
  'gpt-5.6-luna': '5.6 Luna',
  'gpt-5.5': '5.5',
  'deepseek-v4-flash': 'DeepSeek V4 Flash'
}

const stationCategoryKeys: Record<string, string> = {
  daily_development: 'dailyDevelopment',
  hard_problems: 'hardProblems',
  background_automation: 'backgroundAutomation',
  lobster_tasks: 'lobsterTasks'
}

const stationGroups = computed(() => props.data?.station_recommendations ?? [])

const intelligenceGroups = computed<IntelligenceGroup[]>(() => {
  const groups = new Map<string, CodexRadarIntelligenceMetric[]>()
  for (const metric of props.data?.intelligence_recommendations ?? []) {
    const model = metric.model.trim()
    if (!model) continue
    const items = groups.get(model) ?? []
    items.push(metric)
    groups.set(model, items)
  }

  return [...groups.entries()]
    .map(([model, items]) => {
      const sortedItems = [...items].sort((left, right) => {
        const effortDelta = effortRank(left.effort) - effortRank(right.effort)
        return effortDelta || left.effort.localeCompare(right.effort)
      })
      return { model, items: sortedItems, bestKey: bestCombinationKey(sortedItems) }
    })
    .sort((left, right) => {
      const leftOrder = modelOrder[left.model] ?? Number.MAX_SAFE_INTEGER
      const rightOrder = modelOrder[right.model] ?? Number.MAX_SAFE_INTEGER
      return leftOrder - rightOrder || left.model.localeCompare(right.model)
    })
})

const intelligenceItems = computed(() => intelligenceGroups.value.flatMap((group) => group.items))
const hasData = computed(() => stationGroups.value.length > 0 || intelligenceGroups.value.length > 0)
const formattedUpdatedAt = computed(() => formatDateTimeToMinute(props.data?.source_updated_at ?? null))

const summaryCards = computed<SummaryCard[]>(() => {
  const items = intelligenceItems.value
  if (items.length === 0) return []

  const strongest = items.reduce((best, item) => (item.iq > best.iq ? item : best), items[0])
  const fastest = items.reduce((best, item) => {
    if (!isFiniteNumber(item.average_duration_minutes)) return best
    if (!isFiniteNumber(best.average_duration_minutes)) return item
    return item.average_duration_minutes! < best.average_duration_minutes! ? item : best
  }, items[0])
  const balanced = balancedItem(items)

  return [
    {
      key: 'strongest',
      title: String(t('dashboard.modelRecommendations.summary.strongest')),
      item: strongest,
      value: formatIQ(strongest.iq),
      valueLabel: 'IQ',
      tone: 'teal',
      icon: 'brain'
    },
    {
      key: 'balanced',
      title: String(t('dashboard.modelRecommendations.summary.balanced')),
      item: balanced,
      value: formatIQ(balanced.iq),
      valueLabel: 'IQ',
      tone: 'amber',
      icon: 'star'
    },
    {
      key: 'fastest',
      title: String(t('dashboard.modelRecommendations.summary.fastest')),
      item: fastest,
      value: formatDuration(fastest.average_duration_minutes),
      valueLabel: String(t('dashboard.modelRecommendations.time')),
      tone: 'sky',
      icon: 'bolt'
    }
  ]
})

function effortRank(effort: string): number {
  return effortOrder[effort.trim().toLowerCase()] ?? Number.MAX_SAFE_INTEGER
}

function effortLabel(effort: string): string {
  const normalized = effort.trim().toLowerCase()
  return normalized || '-'
}

function reasoningColor(effort: string): string {
  switch (effort.trim().toLowerCase()) {
    case 'low': return '#94a3b8'
    case 'medium': return '#38bdf8'
    case 'high': return '#14b8a6'
    case 'xhigh': return '#8b5cf6'
    case 'max': return '#f59e0b'
    case 'ultra': return '#d4a72c'
    default: return '#64748b'
  }
}

function modelDisplayName(model: string): string {
  const normalized = model.trim().toLowerCase()
  return modelDisplayNames[normalized] ?? (model.trim() || '-')
}

function modelTierColor(model: string): string {
  switch (model.trim().toLowerCase()) {
    case 'gpt-5.6-sol': return '#d4af37'
    case 'gpt-5.6-terra': return '#9ca3af'
    case 'gpt-5.6-luna': return '#b87333'
    case 'gpt-5.5': return '#334155'
    default: return '#cbd5e1'
  }
}

function modelInitial(model: string): string {
  const parts = modelDisplayName(model).split(/\s+/).filter(Boolean)
  return parts.at(-1)?.slice(0, 1).toUpperCase() ?? 'M'
}

function modelVariantName(model: string, effort: string): string {
  const baseName = modelDisplayName(model)
  const normalizedEffort = effort.trim().toLowerCase()
  if (!normalizedEffort) return baseName
  const effortName = normalizedEffort.charAt(0).toUpperCase() + normalizedEffort.slice(1)
  return `${baseName} ${effortName}`
}

function stationTitle(group: CodexRadarStationRecommendationSet): string {
  const key = stationCategoryKeys[group.key]
  return key ? String(t(`dashboard.modelRecommendations.stationCategories.${key}`)) : group.title || group.key
}

function stationItemKey(groupKey: string, item: CodexRadarStationRecommendation): string {
  return `${groupKey}|${item.model}|${item.effort}`
}

function stationPrimary(group: CodexRadarStationRecommendationSet): CodexRadarStationRecommendation | null {
  return group.items[0] ?? null
}

function stationAlternatives(group: CodexRadarStationRecommendationSet): CodexRadarStationRecommendation[] {
  return group.items.slice(1)
}

function intelligenceLevelIndex(group: IntelligenceGroup, item: CodexRadarIntelligenceMetric): string {
  const index = group.items.findIndex((candidate) => intelligenceItemKey(candidate) === intelligenceItemKey(item))
  return index >= 0 ? String(index + 1).padStart(2, '0') : '--'
}

function formatIQ(value: number | null | undefined): string {
  return isFiniteNumber(value) ? value.toFixed(1) : '-'
}

function formatPrice(value: number | null | undefined): string {
  if (!isFiniteNumber(value)) return '-'
  return value < 0.01 ? value.toFixed(4) : value.toFixed(2)
}

function formatDuration(value: number | null | undefined): string {
  if (!isFiniteNumber(value)) return '-'
  const formatted = value < 10 ? value.toFixed(1) : String(Math.round(value))
  return `${formatted}${t('dashboard.modelRecommendations.minutes')}`
}

function isFiniteNumber(value: number | null | undefined): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function bestCombinationKey(items: CodexRadarIntelligenceMetric[]): string | null {
  if (items.length === 0) return null

  const intelligenceValues = items.map((item) => item.iq)
  const priceValues = items.map((item) => item.average_cost_usd)
  const durationValues = items.map((item) => item.average_duration_minutes)
  let bestItem = items[0]
  let bestScore = Number.NEGATIVE_INFINITY

  for (const item of items) {
    const score =
      normalizedScore(item.iq, intelligenceValues) * 0.5 +
      normalizedScore(item.average_cost_usd, priceValues, true) * 0.3 +
      normalizedScore(item.average_duration_minutes, durationValues, true) * 0.2
    if (score > bestScore) {
      bestScore = score
      bestItem = item
    }
  }

  return intelligenceItemKey(bestItem)
}

function balancedItem(items: CodexRadarIntelligenceMetric[]): CodexRadarIntelligenceMetric {
  const priceValues = items.map((item) => item.average_cost_usd)
  const durationValues = items.map((item) => item.average_duration_minutes)
  return items.reduce((best, item) => {
    const itemScore =
      normalizedScore(item.iq, items.map((candidate) => candidate.iq)) * 0.5 +
      normalizedScore(item.average_cost_usd, priceValues, true) * 0.3 +
      normalizedScore(item.average_duration_minutes, durationValues, true) * 0.2
    const bestScore =
      normalizedScore(best.iq, items.map((candidate) => candidate.iq)) * 0.5 +
      normalizedScore(best.average_cost_usd, priceValues, true) * 0.3 +
      normalizedScore(best.average_duration_minutes, durationValues, true) * 0.2
    return itemScore > bestScore ? item : best
  }, items[0])
}

function normalizedScore(value: number | null, values: Array<number | null>, inverse = false): number {
  if (!isFiniteNumber(value)) return 0
  const validValues = values.filter(isFiniteNumber)
  if (validValues.length === 0) return 0

  const min = Math.min(...validValues)
  const max = Math.max(...validValues)
  if (min === max) return 1

  const score = (value - min) / (max - min)
  return inverse ? 1 - score : score
}

function intelligenceItemKey(item: CodexRadarIntelligenceMetric): string {
  return `${item.model}|${item.effort}`
}

function isBest(group: IntelligenceGroup, item: CodexRadarIntelligenceMetric): boolean {
  return group.bestKey === intelligenceItemKey(item)
}
</script>

<style scoped>
.model-recommendations {
  --recommendation-line: rgba(226, 232, 240, 0.82);
}

.summary-card,
.station-lane,
.intelligence-card {
  transition: border-color 160ms ease-out, box-shadow 160ms ease-out, transform 160ms ease-out;
}

.summary-card:hover,
.station-lane:hover {
  border-color: rgba(20, 184, 166, 0.35);
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.06);
}

.intelligence-card:hover {
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.06);
}

.summary-icon {
  display: flex;
  height: 2rem;
  width: 2rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.625rem;
}

.summary-icon[data-tone='teal'] {
  color: #0f766e;
  background: #ccfbf1;
}

.summary-icon[data-tone='amber'] {
  color: #b45309;
  background: #fef3c7;
}

.summary-icon[data-tone='sky'] {
  color: #0369a1;
  background: #e0f2fe;
}

.reasoning-mark {
  display: flex;
  height: 1.75rem;
  width: 1.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
}

.reasoning-mark[data-effort='low'] {
  color: #94a3b8;
  background: #f1f5f9;
}

.reasoning-mark[data-effort='medium'] {
  color: #38bdf8;
  background: #e0f2fe;
}

.reasoning-mark[data-effort='high'] {
  color: #14b8a6;
  background: #ccfbf1;
}

.reasoning-mark[data-effort='xhigh'] {
  color: #8b5cf6;
  background: #ede9fe;
}

.reasoning-mark[data-effort='max'] {
  color: #f59e0b;
  background: #fef3c7;
}

.reasoning-mark[data-effort='ultra'] {
  color: #d4a72c;
  background: #fef3c7;
}

.intelligence-model-name {
  color: #111827;
}

.summary-meta {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.summary-meta-item {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: #94a3b8;
  font-size: 0.625rem;
}

.summary-meta-item strong {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.6875rem;
  font-weight: 600;
}

.summary-meta-price strong {
  color: #475569;
}

.summary-meta-time strong {
  color: #64748b;
  font-weight: 500;
}

.section-count {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border-radius: 9999px;
  background: #f8fafc;
  padding: 0.35rem 0.6rem;
  font-size: 0.6875rem;
  font-weight: 500;
  color: #64748b;
}

.station-index {
  display: flex;
  height: 1.75rem;
  width: 1.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.55rem;
  background: #ccfbf1;
  color: #0f766e;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.625rem;
  font-weight: 700;
}

.station-lane:nth-child(2) .station-index {
  color: #0369a1;
  background: #e0f2fe;
}

.station-lane:nth-child(3) .station-index {
  color: #7c3aed;
  background: #ede9fe;
}

.station-lane:nth-child(4) .station-index {
  color: #b45309;
  background: #fef3c7;
}

.effort-pill {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  border-radius: 0.35rem;
  background: #f1f5f9;
  padding: 0.2rem 0.45rem;
  color: #475569;
  font-size: 0.625rem;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.effort-pill-strong {
  color: #0f766e;
  background: #f0fdfa;
}

.station-score {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.75rem;
}

.recommendation-callout {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border-radius: 0.4rem;
  background: #f0fdfa;
  padding: 0.3rem 0.45rem;
  color: #0f766e;
  font-size: 0.625rem;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.station-metric-grid {
  display: grid;
  gap: 0.75rem;
}

.station-metric-cell {
  min-width: 0;
}

.station-metric-cell + .station-metric-cell {
  border-left: 1px solid var(--recommendation-line);
  padding-left: 0.75rem;
}

.station-metric-cell dt,
.intelligence-metric dt {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  color: #94a3b8;
  font-size: 0.625rem;
}

.station-metric-cell dd,
.intelligence-metric dd {
  margin-top: 0.25rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #334155;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  font-weight: 600;
}

.station-metric-price dd,
.intelligence-metric-price dd {
  color: #475569;
  font-size: 0.75rem;
}

.station-metric-time dd,
.intelligence-metric-time dd {
  color: #94a3b8;
  font-size: 0.6875rem;
  font-weight: 500;
}

.station-alternative-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: #94a3b8;
  font-size: 0.625rem;
}

.station-alternative-meta span:last-child,
.station-alternative-price {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}

.station-alternative-price {
  color: #94a3b8;
  font-size: 0.625rem;
  font-weight: 500;
}

.intelligence-level-index {
  display: flex;
  height: 1.75rem;
  width: 1.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  background: #f0fdfa;
  color: #0f766e;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.625rem;
  font-weight: 700;
}

.intelligence-iq-hero {
  display: flex;
  min-height: 2.25rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  border-bottom: 1px solid var(--recommendation-line);
  padding-bottom: 0.55rem;
}

.intelligence-iq-label {
  color: #0f766e;
  font-size: 0.625rem;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.intelligence-iq-value {
  color: #0f766e;
  font-variant-numeric: tabular-nums;
}

.intelligence-model-line {
  display: flex;
  min-height: 1.9rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding-top: 0.5rem;
}

.intelligence-variant-name {
  min-width: 0;
  color: #475569;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.6875rem;
  font-weight: 600;
  line-height: 1.1;
}

.intelligence-metric + .intelligence-metric {
  border-left: 1px solid var(--recommendation-line);
  padding-left: 0.75rem;
}

.intelligence-level-count {
  color: #94a3b8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.625rem;
  font-variant-numeric: tabular-nums;
}

.best-note {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: #b45309;
  font-weight: 600;
}

.station-alternatives {
  border-radius: 0.625rem;
  background: #f8fafc;
  padding-right: 0.7rem;
  padding-bottom: 0.35rem;
  padding-left: 0.7rem;
}

.station-alternatives-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  color: #64748b;
  font-size: 0.625rem;
  font-weight: 600;
}

.station-alternatives-title {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.station-alternatives-count {
  display: inline-flex;
  min-width: 1.2rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: #fff;
  padding: 0.15rem 0.35rem;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.5625rem;
}

.station-alt-index {
  display: flex;
  height: 1.4rem;
  width: 1.4rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border: 1px solid #e2e8f0;
  border-radius: 0.4rem;
  background: #fff;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.5625rem;
  font-weight: 700;
}

.station-alternative + .station-alternative {
  border-top: 1px solid var(--recommendation-line);
}

.model-mark {
  display: flex;
  height: 1.75rem;
  width: 1.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  background: #f1f5f9;
  color: #475569;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.625rem;
  font-weight: 700;
}

.intelligence-card-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.5rem;
}

.intelligence-card {
  border: 1px solid var(--model-tier-color);
  border-top-width: 7px;
}

.best-stamp {
  height: 1.7rem;
  width: 1.7rem;
  border: 1px dashed #d97706;
  border-radius: 9999px;
  color: #d97706;
  background: #fffbeb;
  box-shadow: 0 2px 5px rgba(180, 83, 9, 0.12);
  transform: rotate(8deg);
}

:global(.dark) .model-recommendations {
  --recommendation-line: rgba(51, 65, 85, 0.8);
}

:global(.dark) .summary-card:hover,
:global(.dark) .station-lane:hover,
:global(.dark) .intelligence-card:hover {
  box-shadow: 0 8px 20px rgba(2, 6, 23, 0.24);
}

:global(.dark) .summary-icon[data-tone='teal'] {
  color: #5eead4;
  background: rgba(19, 78, 74, 0.45);
}

:global(.dark) .summary-icon[data-tone='amber'] {
  color: #fcd34d;
  background: rgba(120, 53, 15, 0.35);
}

:global(.dark) .summary-icon[data-tone='sky'] {
  color: #7dd3fc;
  background: rgba(7, 89, 133, 0.35);
}

:global(.dark) .reasoning-mark[data-effort='low'] {
  color: #cbd5e1;
  background: #1e293b;
}

:global(.dark) .reasoning-mark[data-effort='medium'] {
  color: #7dd3fc;
  background: rgba(7, 89, 133, 0.35);
}

:global(.dark) .reasoning-mark[data-effort='high'] {
  color: #5eead4;
  background: rgba(19, 78, 74, 0.45);
}

:global(.dark) .reasoning-mark[data-effort='xhigh'] {
  color: #c4b5fd;
  background: rgba(76, 29, 149, 0.32);
}

:global(.dark) .reasoning-mark[data-effort='max'] {
  color: #fcd34d;
  background: rgba(120, 53, 15, 0.35);
}

:global(.dark) .reasoning-mark[data-effort='ultra'] {
  color: #fda4af;
  background: rgba(159, 18, 57, 0.32);
}

:global(.dark) .section-count,
:global(.dark) .effort-pill,
:global(.dark) .model-mark {
  background: #1e293b;
  color: #cbd5e1;
}

:global(.dark) .station-index {
  color: #5eead4;
  background: rgba(19, 78, 74, 0.45);
}

:global(.dark) .station-lane:nth-child(2) .station-index {
  color: #7dd3fc;
  background: rgba(7, 89, 133, 0.35);
}

:global(.dark) .station-lane:nth-child(3) .station-index {
  color: #c4b5fd;
  background: rgba(76, 29, 149, 0.32);
}

:global(.dark) .station-lane:nth-child(4) .station-index {
  color: #fcd34d;
  background: rgba(120, 53, 15, 0.35);
}

:global(.dark) .effort-pill-strong {
  color: #5eead4;
  background: rgba(19, 78, 74, 0.35);
}

:global(.dark) .best-star {
  color: #fcd34d;
  background: rgba(120, 53, 15, 0.32);
}

:global(.dark) .station-alternatives {
  background: rgba(15, 23, 42, 0.42);
}

:global(.dark) .station-alternatives-count,
:global(.dark) .station-alt-index {
  border-color: #334155;
  background: #1e293b;
  color: #cbd5e1;
}

:global(.dark) .recommendation-callout,
:global(.dark) .intelligence-level-index {
  color: #5eead4;
  background: rgba(19, 78, 74, 0.35);
}

:global(.dark) .station-metric-cell dd,
:global(.dark) .intelligence-metric dd {
  color: #e2e8f0;
}

:global(.dark) .summary-meta-price strong,
:global(.dark) .station-metric-price dd,
:global(.dark) .intelligence-metric-price dd,
:global(.dark) .intelligence-variant-name,
:global(.dark) .intelligence-iq-label,
:global(.dark) .intelligence-iq-value {
  color: #cbd5e1;
}

:global(.dark) .intelligence-model-name {
  color: #f8fafc;
}

:global(.dark) .intelligence-card {
  border-color: color-mix(in srgb, var(--model-tier-color) 78%, #0f172a);
}

:global(.dark) .best-stamp {
  border-color: #fcd34d;
  color: #fcd34d;
  background: rgba(120, 53, 15, 0.2);
  box-shadow: 0 2px 5px rgba(2, 6, 23, 0.28);
}

:global(.dark) .summary-meta-time strong,
:global(.dark) .station-metric-time dd,
:global(.dark) .intelligence-metric-time dd,
:global(.dark) .station-alternative-meta,
:global(.dark) .station-alternative-price {
  color: #94a3b8;
}

@media (min-width: 640px) {
  .intelligence-card-grid {
    grid-template-columns: repeat(auto-fill, minmax(170px, 214px));
  }
}

@media (prefers-reduced-motion: reduce) {
  .summary-card,
  .station-lane,
  .intelligence-card {
    transition: none;
  }
}
</style>
