<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpsInvestigationFinding, OpsInvestigationResponse } from '@/api/admin/ops'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  data: OpsInvestigationResponse | null
  loading: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (event: 'openErrorDetails', owner: 'request' | 'upstream'): void
}>()
const { t } = useI18n()

const findings = computed(() => props.data?.findings ?? [])
const hasData = computed(() => findings.value.length > 0)

function severityClass(severity: string): string {
  if (severity === 'critical') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (severity === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
}

function formatCount(value: number | undefined): string {
  return new Intl.NumberFormat().format(Math.max(0, Number(value) || 0))
}

function formatMs(value: number | undefined): string {
  return `${formatCount(value)} ms`
}

function openFinding(finding: OpsInvestigationFinding) {
  emit('openErrorDetails', finding.owner === 'provider' ? 'upstream' : 'request')
}
</script>

<template>
  <section class="flex h-full max-h-[420px] flex-col overflow-hidden rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.investigation.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.investigation.subtitle') }}</p>
      </div>
      <span v-if="data" class="shrink-0 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.ops.investigation.totalErrors', { count: formatCount(data.total_errors) }) }}
      </span>
    </div>

    <div v-if="loading && !data" class="py-8 text-center text-sm text-gray-400">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="!hasData" :title="t('admin.ops.investigation.emptyTitle')" :description="t('admin.ops.investigation.emptyDescription')" />
    <div v-else class="min-h-0 flex-1 divide-y divide-gray-100 overflow-y-auto pr-1 dark:divide-dark-700">
      <button
        v-for="finding in findings"
        :key="`${finding.rule}-${finding.platform}-${finding.group_id}-${finding.status_code}`"
        type="button"
        class="flex w-full items-start gap-3 py-3 text-left first:pt-0 last:pb-0 hover:bg-gray-50 dark:hover:bg-dark-700/40"
        @click="openFinding(finding)"
      >
        <span :class="['mt-0.5 shrink-0 rounded px-2 py-0.5 text-[11px] font-semibold', severityClass(finding.severity)]">
          {{ t(`admin.ops.investigation.severity.${finding.severity}`) }}
        </span>
        <span class="min-w-0 flex-1">
          <span class="block text-sm font-semibold text-gray-900 dark:text-white">
            {{ t(`admin.ops.investigation.rules.${finding.rule}.title`) }}
          </span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
            <template v-if="finding.kind === 'latency'">
              {{ t('admin.ops.investigation.latencyEvidence', { current: formatMs(finding.current_value_ms), baseline: formatMs(finding.baseline_value_ms), change: finding.change_percent }) }}
            </template>
            <template v-else>
              {{ t('admin.ops.investigation.errorEvidence', { current: formatCount(finding.current_count), baseline: formatCount(finding.baseline_count), change: finding.change_percent, share: finding.share_percent }) }}
            </template>
          </span>
          <span v-if="finding.platform || finding.group_id" class="mt-1 block text-[11px] text-gray-400">
            <template v-if="finding.platform">{{ finding.platform }}</template>
            <template v-if="finding.platform && finding.group_id"> · </template>
            <template v-if="finding.group_id">group={{ finding.group_id }}</template>
          </span>
        </span>
        <span class="shrink-0 text-xs font-medium text-primary-600 dark:text-primary-400">
          {{ t(`admin.ops.investigation.rules.${finding.rule}.action`) }}
        </span>
      </button>
    </div>
  </section>
</template>
