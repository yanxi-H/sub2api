<template>
  <section class="card overflow-hidden" data-testid="rate-limit-overview">
    <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700/70">
      <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <span class="flex h-7 w-7 items-center justify-center rounded-md bg-teal-100 text-teal-600 dark:bg-teal-900/30 dark:text-teal-400">
              <Icon name="chartBar" size="sm" :stroke-width="2" />
            </span>
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.rateLimits.title') }}
              </h2>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.dashboard.rateLimits.total', { count: activeTotal }) }}
              </p>
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
          <div class="inline-flex h-9 rounded-lg bg-gray-100 p-1 dark:bg-dark-700" role="tablist">
            <button
              type="button"
              role="tab"
              data-testid="accounts-tab"
              class="min-w-[112px] rounded-md px-3 text-xs font-medium transition-colors"
              :class="activeTab === 'accounts' ? activeTabClass : inactiveTabClass"
              :aria-selected="activeTab === 'accounts'"
              @click="selectTab('accounts')"
            >
              {{ t('admin.dashboard.rateLimits.aiAccounts') }}
            </button>
            <button
              type="button"
              role="tab"
              data-testid="keys-tab"
              class="min-w-[96px] rounded-md px-3 text-xs font-medium transition-colors"
              :class="activeTab === 'keys' ? activeTabClass : inactiveTabClass"
              :aria-selected="activeTab === 'keys'"
              @click="selectTab('keys')"
            >
              {{ t('admin.dashboard.rateLimits.apiKeys') }}
            </button>
          </div>

          <div class="flex min-w-0 items-center gap-2">
            <div class="min-w-0 flex-1 sm:w-52 sm:flex-none">
              <SearchInput
                v-model="activeSearch"
                :placeholder="t('admin.dashboard.rateLimits.searchPlaceholder')"
                @search="handleSearch"
              />
            </div>
            <button
              type="button"
              class="btn btn-secondary h-9 w-9 flex-shrink-0 p-0"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              :disabled="activeLoading"
              @click="loadActiveTab"
            >
              <Icon name="refresh" size="sm" :class="activeLoading ? 'animate-spin' : ''" />
            </button>
            <button
              v-if="activeTab === 'accounts'"
              type="button"
              data-testid="live-refresh"
              class="btn btn-secondary h-9 flex-shrink-0 rounded-lg px-3 py-0 text-xs"
              :title="t('admin.dashboard.rateLimits.liveRefreshHint')"
              :disabled="liveRefreshing || activeLoading || refreshableAccountIds.length === 0"
              @click="refreshUpstream"
            >
              <Icon
                :name="liveRefreshing ? 'refresh' : 'cloud'"
                size="sm"
                :class="liveRefreshing ? 'animate-spin' : ''"
              />
              <span class="hidden sm:inline">{{ t('admin.dashboard.rateLimits.liveRefresh') }}</span>
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="liveMessage"
        class="mt-3 flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-700/60 dark:text-gray-300"
        role="status"
      >
        <Icon name="check" size="xs" class="text-emerald-500" />
        {{ liveMessage }}
      </div>
    </div>

    <div v-if="activeLoading && activeItems.length === 0" class="flex h-48 items-center justify-center">
      <LoadingSpinner size="md" />
    </div>
    <div v-else-if="activeError" class="flex h-48 flex-col items-center justify-center gap-3 px-4 text-center">
      <p class="text-sm text-red-600 dark:text-red-400">{{ activeError }}</p>
      <button type="button" class="btn btn-secondary py-2 text-xs" @click="loadActiveTab">
        <Icon name="refresh" size="sm" />
        {{ t('admin.dashboard.rateLimits.retry') }}
      </button>
    </div>
    <div v-else-if="activeItems.length === 0" class="flex h-48 flex-col items-center justify-center gap-2 px-4 text-center">
      <Icon :name="activeTab === 'accounts' ? 'server' : 'key'" size="lg" class="text-gray-300 dark:text-gray-600" />
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.dashboard.rateLimits.empty') }}
      </p>
    </div>

    <div v-else class="p-3 sm:p-4" :aria-busy="activeLoading">
      <template v-if="activeTab === 'accounts'">
        <div class="grid gap-3 md:grid-cols-2">
          <article
            v-for="item in accountItems"
            :key="item.id"
            class="overflow-hidden rounded-lg border border-gray-200 bg-white transition-colors hover:border-gray-300 dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-dark-600"
            data-testid="account-row"
          >
            <div class="flex min-w-0 items-start justify-between gap-3 border-b border-gray-100 px-3.5 py-3 dark:border-dark-700/70">
              <div class="min-w-0">
                <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="item.name">
                  {{ item.name }}
                </p>
                <div class="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                  <span class="capitalize">{{ item.platform }}</span>
                  <span class="text-gray-300 dark:text-gray-600">/</span>
                  <span class="truncate">{{ item.type }}</span>
                  <span v-if="item.updated_at" class="text-gray-300 dark:text-gray-600">/</span>
                  <span v-if="item.updated_at" class="truncate" :title="formatDateTime(item.updated_at)">
                    {{ formatRelativeTime(item.updated_at) }}
                  </span>
                </div>
                <p v-if="item.refresh_error" class="mt-1 truncate text-[11px] text-red-500">
                  {{ refreshErrorLabel(item.refresh_error) }}
                </p>
              </div>
              <span class="flex-shrink-0 rounded-md px-2 py-1 text-[11px] font-medium" :class="statusClass(item.status)">
                {{ statusLabel(item.status) }}
              </span>
            </div>
            <div class="grid grid-cols-2 gap-3 px-3.5 py-3">
              <div class="min-w-0">
                <p class="mb-1.5 text-[11px] font-medium text-gray-400">{{ t('admin.dashboard.rateLimits.fiveHour') }}</p>
                <RateLimitGauge
                  :utilization="item.five_hour?.utilization"
                  :resets-at="item.five_hour?.resets_at"
                />
              </div>
              <div class="min-w-0">
                <p class="mb-1.5 text-[11px] font-medium text-gray-400">{{ t('admin.dashboard.rateLimits.sevenDay') }}</p>
                <RateLimitGauge
                  :utilization="item.seven_day?.utilization"
                  :resets-at="item.seven_day?.resets_at"
                />
              </div>
            </div>
          </article>
        </div>
      </template>

      <template v-else>
      <div class="grid gap-3 lg:grid-cols-2">
        <section
          v-for="group in keyGroups"
          :key="group.key"
          data-testid="key-group"
          class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800/40"
        >
        <div class="flex min-w-0 flex-col gap-2 border-b border-gray-100 bg-gray-50/80 px-3.5 py-2.5 dark:border-dark-700/70 dark:bg-dark-800/70 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
          <div class="flex min-w-0 items-center gap-2">
            <span class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-md bg-white text-gray-500 shadow-sm dark:bg-dark-700 dark:text-gray-300">
              <Icon name="grid" size="xs" />
            </span>
            <div class="min-w-0">
              <p
                data-testid="key-group-owner"
                class="truncate text-xs font-semibold text-gray-800 dark:text-gray-100"
                :title="group.title"
              >
                {{ group.name }}
              </p>
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('admin.dashboard.rateLimits.keyCount', { count: group.items.length }) }}
              </p>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-1.5 sm:justify-end">
            <span class="mr-1 text-xs font-medium text-gray-500 dark:text-gray-400 sm:flex-shrink-0">
              {{ t('admin.dashboard.rateLimits.recentlyActive', { count: group.activeCount }) }}
            </span>
            <button
              type="button"
              class="btn btn-secondary h-8 px-2 text-[11px]"
              data-testid="sync-7d-window-button"
              :aria-expanded="isBatchActionOpen(group.key, 'sync')"
              @click="toggleBatchAction(group, 'sync')"
            >
              <Icon name="sync" size="xs" />
              {{ t('admin.dashboard.rateLimits.sync7dWindow') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary h-8 px-2 text-[11px] text-red-600 dark:text-red-400"
              data-testid="reset-7d-usage-button"
              :aria-expanded="isBatchActionOpen(group.key, 'reset')"
              @click="toggleBatchAction(group, 'reset')"
            >
              <Icon name="refresh" size="xs" />
              {{ t('admin.dashboard.rateLimits.reset7dUsage') }}
            </button>
          </div>
        </div>
        <div
          v-if="openBatchGroupKey === group.key"
          class="border-b border-gray-100 bg-white px-3.5 py-3 dark:border-dark-700/70 dark:bg-dark-800/40"
          data-testid="key-batch-panel"
        >
          <div v-if="openBatchAction === 'sync'" class="mb-3">
            <label class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-200">
              {{ t('admin.dashboard.rateLimits.upstreamAccount') }}
            </label>
            <select
              v-model.number="selectedSyncAccountID"
              class="input h-9 w-full text-xs sm:max-w-xl"
              data-testid="sync-account-select"
              :disabled="batchAccountsLoading || batchSubmitting"
            >
              <option :value="null">
                {{ batchAccountsLoading ? t('common.loading') : t('admin.dashboard.rateLimits.selectUpstreamAccount') }}
              </option>
              <option v-for="account in syncAccountOptions" :key="account.id" :value="account.id">
                {{ account.label }}
              </option>
            </select>
            <p v-if="!batchAccountsLoading && syncAccountOptions.length === 0" class="mt-1 text-[11px] text-amber-600 dark:text-amber-400">
              {{ t('admin.dashboard.rateLimits.noSyncableAccount') }}
            </p>
          </div>

          <div class="flex items-center justify-between gap-3 border-b border-gray-100 pb-2 dark:border-dark-700/70">
            <label class="flex cursor-pointer items-center gap-2 text-xs font-medium text-gray-700 dark:text-gray-200">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-testid="select-all-keys"
                :checked="areAllGroupKeysSelected(group)"
                :disabled="batchSubmitting"
                @change="toggleAllGroupKeys(group)"
              />
              {{ t('common.selectAll') }}
            </label>
            <span class="text-[11px] text-gray-500 dark:text-gray-400">
              {{ t('admin.dashboard.rateLimits.selectedKeys', { selected: selectedBatchKeyIDs.length, total: group.items.length }) }}
            </span>
          </div>
          <div class="max-h-52 overflow-y-auto py-1">
            <label
              v-for="item in group.items"
              :key="`select-${item.id}`"
              class="flex cursor-pointer items-center gap-2 border-b border-gray-50 py-2 last:border-0 dark:border-dark-700/50"
            >
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                data-testid="batch-key-checkbox"
                :checked="selectedBatchKeyIDs.includes(item.id)"
                :disabled="batchSubmitting"
                @change="toggleBatchKey(item.id)"
              />
              <span class="min-w-0 flex-1 truncate text-xs font-medium text-gray-800 dark:text-gray-100">{{ item.name }}</span>
              <span class="truncate text-[11px] text-gray-400">{{ ownerLabel(item) }}</span>
            </label>
          </div>
          <div class="mt-3 flex justify-end gap-2">
            <button type="button" class="btn btn-secondary h-8 px-3 text-xs" :disabled="batchSubmitting" @click="closeBatchAction">
              {{ t('common.cancel') }}
            </button>
            <button
              type="button"
              class="btn btn-primary h-8 px-3 text-xs"
              data-testid="confirm-batch-action"
              :disabled="!canSubmitBatchAction"
              @click="submitBatchAction"
            >
              <LoadingSpinner v-if="batchSubmitting" size="sm" />
              {{ t('common.confirm') }}
            </button>
          </div>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700/70">
          <div
            v-for="item in group.items"
            :key="item.id"
            class="grid key-row-cols gap-x-3 gap-y-2 px-3.5 py-2.5 transition-colors hover:bg-gray-50/70 dark:hover:bg-dark-700/30"
            data-testid="key-row"
          >
          <div class="flex min-w-0 items-center gap-2">
            <span
              data-testid="key-activity"
              class="h-2 w-2 flex-shrink-0 rounded-full"
              :class="isKeyActive(item) ? 'bg-emerald-500 ring-4 ring-emerald-500/10' : 'bg-gray-300 dark:bg-dark-500'"
              :title="isKeyActive(item) ? t('admin.dashboard.rateLimits.activeWithinFiveMinutes') : t('admin.dashboard.rateLimits.inactiveWithinFiveMinutes')"
            ></span>
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="item.name">
                {{ item.name }}
              </p>
              <p class="truncate text-[11px] text-gray-400 dark:text-gray-500" :title="ownerTitle(item)">
                {{ ownerLabel(item) }}
              </p>
            </div>
          </div>
          <div class="flex items-center justify-end">
            <span class="rounded-md px-2 py-1 text-[11px] font-medium" :class="statusClass(item.status)">
              {{ statusLabel(item.status) }}
            </span>
          </div>
          <div class="min-w-0">
            <p class="mb-1 text-[11px] font-medium text-gray-400">{{ t('admin.dashboard.rateLimits.fiveHour') }}</p>
            <div v-if="item.rate_limit_5h > 0" class="min-w-0">
              <div class="flex items-center justify-between gap-2 text-xs">
                <span class="truncate font-medium text-gray-700 dark:text-gray-200">{{ formatKeyUsage(item.usage_5h, item.rate_limit_5h) }}</span>
                <span class="flex-shrink-0 text-gray-400">{{ formatPercent(keyUtilization(item.usage_5h, item.rate_limit_5h)) }}</span>
              </div>
              <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div class="h-full rounded-full" :class="utilizationClass(keyUtilization(item.usage_5h, item.rate_limit_5h))" :style="{ width: `${clampedUtilization(keyUtilization(item.usage_5h, item.rate_limit_5h))}%` }"></div>
              </div>
              <p
                v-if="item.reset_5h_at"
                class="mt-0.5 truncate text-[10px] leading-4 text-gray-400 dark:text-gray-500"
                :title="formatDateTime(item.reset_5h_at)"
              >
                {{ t('admin.dashboard.rateLimits.resetsAt', { time: formatDateTime(item.reset_5h_at) }) }}
              </p>
            </div>
            <span v-else class="text-xs text-gray-400 dark:text-gray-500">{{ t('admin.dashboard.rateLimits.unlimited') }}</span>
          </div>
          <div class="min-w-0">
            <p class="mb-1 text-[11px] font-medium text-gray-400">{{ t('admin.dashboard.rateLimits.sevenDay') }}</p>
            <div v-if="item.rate_limit_7d > 0" class="min-w-0">
              <div class="flex items-center justify-between gap-2 text-xs">
                <span class="truncate font-medium text-gray-700 dark:text-gray-200">{{ formatKeyUsage(item.usage_7d, item.rate_limit_7d) }}</span>
                <span class="flex-shrink-0 text-gray-400">{{ formatPercent(keyUtilization(item.usage_7d, item.rate_limit_7d)) }}</span>
              </div>
              <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div class="h-full rounded-full" :class="utilizationClass(keyUtilization(item.usage_7d, item.rate_limit_7d))" :style="{ width: `${clampedUtilization(keyUtilization(item.usage_7d, item.rate_limit_7d))}%` }"></div>
              </div>
              <p
                v-if="item.reset_7d_at"
                class="mt-0.5 truncate text-[10px] leading-4 text-gray-400 dark:text-gray-500"
                :title="formatDateTime(item.reset_7d_at)"
              >
                {{ t('admin.dashboard.rateLimits.resetsAt', { time: formatDateTime(item.reset_7d_at) }) }}
              </p>
            </div>
            <span v-else class="text-xs text-gray-400 dark:text-gray-500">{{ t('admin.dashboard.rateLimits.unlimited') }}</span>
          </div>
          </div>
        </div>
        </section>
      </div>
      </template>
    </div>

    <Pagination
      v-if="activeTotal > activePageSize"
      :total="activeTotal"
      :page="activePage"
      :page-size="activePageSize"
      :show-page-size-selector="false"
      @update:page="changePage"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import keysAPI from '@/api/keys'
import type { AccountUsageWindowItem } from '@/api/admin/accounts'
import type { Account, ApiKey } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import RateLimitGauge from './RateLimitGauge.vue'

type PanelTab = 'accounts' | 'keys'
type KeyBatchAction = 'sync' | 'reset'

interface SyncAccountOption {
  id: number
  label: string
}

interface ApiKeyRateLimitGroup {
  key: string
  id: number | null
  name: string
  title: string
  activeCount: number
  items: ApiKey[]
}

const { t } = useI18n()
const appStore = useAppStore()
const accountPageSize = 10
const keyPageSize = 1000
const activeTab = ref<PanelTab>('keys')
const accountItems = ref<AccountUsageWindowItem[]>([])
const keyItems = ref<ApiKey[]>([])
const accountTotal = ref(0)
const keyTotal = ref(0)
const accountPage = ref(1)
const keyPage = ref(1)
const accountSearch = ref('')
const keySearch = ref('')
const accountLoading = ref(false)
const keyLoading = ref(false)
const accountLoaded = ref(false)
const keyLoaded = ref(false)
const accountError = ref('')
const keyError = ref('')
const liveRefreshing = ref(false)
const liveMessage = ref('')
const openBatchGroupKey = ref<string | null>(null)
const openBatchGroup = ref<ApiKeyRateLimitGroup | null>(null)
const openBatchAction = ref<KeyBatchAction | null>(null)
const selectedBatchKeyIDs = ref<number[]>([])
const selectedSyncAccountID = ref<number | null>(null)
const syncAccountOptions = ref<SyncAccountOption[]>([])
const batchAccountsLoading = ref(false)
const batchSubmitting = ref(false)
const activityNow = ref(Date.now())
let accountController: AbortController | null = null
let keyController: AbortController | null = null
let activityTimer: ReturnType<typeof setInterval> | null = null

const activeTabClass = 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white'
const inactiveTabClass = 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
const activeItems = computed(() => activeTab.value === 'accounts' ? accountItems.value : keyItems.value)
const activeTotal = computed(() => activeTab.value === 'accounts' ? accountTotal.value : keyTotal.value)
const activePage = computed(() => activeTab.value === 'accounts' ? accountPage.value : keyPage.value)
const activePageSize = computed(() => activeTab.value === 'accounts' ? accountPageSize : keyPageSize)
const activeLoading = computed(() => activeTab.value === 'accounts' ? accountLoading.value : keyLoading.value)
const activeError = computed(() => activeTab.value === 'accounts' ? accountError.value : keyError.value)
const keyGroups = computed<ApiKeyRateLimitGroup[]>(() => {
  const groups = new Map<string, ApiKeyRateLimitGroup>()

  for (const key of keyItems.value) {
    const groupID = key.group_id ?? null
    const groupKey = groupID === null ? 'unassigned' : `group-${groupID}`
    const groupName = key.group?.name?.trim() || t('admin.dashboard.rateLimits.unassignedGroup')
    let group = groups.get(groupKey)
    if (!group) {
      group = {
        key: groupKey,
        id: groupID,
        name: groupName,
        title: groupID === null
          ? t('admin.dashboard.rateLimits.unassignedGroup')
          : `${groupName} #${groupID}`,
        activeCount: 0,
        items: []
      }
      groups.set(groupKey, group)
    }
    if (isKeyActive(key)) group.activeCount += 1
    group.items.push(key)
  }

  return Array.from(groups.values())
    .map((group) => ({
      ...group,
      items: [...group.items].sort(compareKeyUsage)
    }))
    .sort((a, b) => a.name.localeCompare(b.name) || (a.id ?? Number.MAX_SAFE_INTEGER) - (b.id ?? Number.MAX_SAFE_INTEGER))
})
const activeSearch = computed({
  get: () => activeTab.value === 'accounts' ? accountSearch.value : keySearch.value,
  set: (value: string) => {
    if (activeTab.value === 'accounts') accountSearch.value = value
    else keySearch.value = value
  }
})
const refreshableAccountIds = computed(() => accountItems.value.filter((item) => item.supports_live_refresh).map((item) => item.id))
const canSubmitBatchAction = computed(() => {
  if (batchSubmitting.value || selectedBatchKeyIDs.value.length === 0) return false
  return openBatchAction.value === 'reset' || (openBatchAction.value === 'sync' && selectedSyncAccountID.value !== null)
})

function isBatchActionOpen(groupKey: string, action: KeyBatchAction): boolean {
  return openBatchGroupKey.value === groupKey && openBatchAction.value === action
}

function closeBatchAction(): void {
  openBatchGroupKey.value = null
  openBatchGroup.value = null
  openBatchAction.value = null
  selectedBatchKeyIDs.value = []
  selectedSyncAccountID.value = null
  syncAccountOptions.value = []
}

async function toggleBatchAction(group: ApiKeyRateLimitGroup, action: KeyBatchAction): Promise<void> {
  if (isBatchActionOpen(group.key, action)) {
    closeBatchAction()
    return
  }
  openBatchGroupKey.value = group.key
  openBatchGroup.value = group
  openBatchAction.value = action
  selectedBatchKeyIDs.value = []
  selectedSyncAccountID.value = null
  syncAccountOptions.value = []
  if (action === 'sync') await loadSyncAccounts(group)
}

async function loadSyncAccounts(group: ApiKeyRateLimitGroup): Promise<void> {
  batchAccountsLoading.value = true
  try {
    const response = await adminAPI.accounts.list(1, 1000, {
      group: group.id === null ? 'ungrouped' : String(group.id),
      sort_by: 'name',
      sort_order: 'asc',
      lite: '1'
    })
    if (openBatchGroupKey.value !== group.key || openBatchAction.value !== 'sync') return
    syncAccountOptions.value = response.items
      .map((account) => toSyncAccountOption(account))
      .filter((account): account is SyncAccountOption => account !== null)
  } catch (error: any) {
    if (openBatchGroupKey.value === group.key) {
      appStore.showError(error?.message || t('admin.dashboard.rateLimits.loadAccountsFailed'))
    }
  } finally {
    if (openBatchGroupKey.value === group.key) batchAccountsLoading.value = false
  }
}

function toSyncAccountOption(account: Account): SyncAccountOption | null {
  const resetAt = account7dResetAt(account)
  if (!resetAt) return null
  return {
    id: account.id,
    label: `#${account.id} ${account.name} · ${account.platform} · ${formatDateTime(resetAt)}`
  }
}

function account7dResetAt(account: Account): string | null {
  const extra = account.extra || {}
  const candidates = [extra.codex_7d_reset_at, extra.passive_usage_7d_reset, extra.quota_weekly_reset_at]
  const now = Date.now()
  for (const value of candidates) {
    const timestamp = typeof value === 'number' ? value * 1000 : typeof value === 'string' ? Date.parse(value) : Number.NaN
    if (Number.isFinite(timestamp) && timestamp > now) return new Date(timestamp).toISOString()
  }
  return null
}

function toggleBatchKey(keyID: number): void {
  selectedBatchKeyIDs.value = selectedBatchKeyIDs.value.includes(keyID)
    ? selectedBatchKeyIDs.value.filter((id) => id !== keyID)
    : [...selectedBatchKeyIDs.value, keyID]
}

function areAllGroupKeysSelected(group: ApiKeyRateLimitGroup): boolean {
  return group.items.length > 0 && group.items.every((item) => selectedBatchKeyIDs.value.includes(item.id))
}

function toggleAllGroupKeys(group: ApiKeyRateLimitGroup): void {
  selectedBatchKeyIDs.value = areAllGroupKeysSelected(group) ? [] : group.items.map((item) => item.id)
}

async function submitBatchAction(): Promise<void> {
  if (!canSubmitBatchAction.value || !openBatchAction.value || !openBatchGroup.value) return
  batchSubmitting.value = true
  try {
    const count = selectedBatchKeyIDs.value.length
    if (openBatchAction.value === 'sync') {
      await adminAPI.apiKeys.batchSync7dWindow(selectedBatchKeyIDs.value, openBatchGroup.value.id, selectedSyncAccountID.value as number)
      appStore.showSuccess(t('admin.dashboard.rateLimits.sync7dSuccess', { count }))
    } else {
      await adminAPI.apiKeys.batchReset7dUsage(selectedBatchKeyIDs.value, openBatchGroup.value.id)
      appStore.showSuccess(t('admin.dashboard.rateLimits.reset7dSuccess', { count }))
    }
    closeBatchAction()
    await loadKeys()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.dashboard.rateLimits.batchActionFailed'))
  } finally {
    batchSubmitting.value = false
  }
}

async function loadAccounts(): Promise<void> {
  accountController?.abort()
  const controller = new AbortController()
  accountController = controller
  accountLoading.value = true
  accountError.value = ''
  liveMessage.value = ''
  try {
    const response = await adminAPI.accounts.listUsageWindows(
      accountPage.value,
      accountPageSize,
      accountSearch.value.trim(),
      { signal: controller.signal }
    )
    if (accountController !== controller) return
    accountItems.value = response.items
    accountTotal.value = response.total
    accountLoaded.value = true
  } catch (error: any) {
    if (accountController !== controller) return
    if (error?.name === 'CanceledError' || error?.name === 'AbortError') return
    accountError.value = error?.message || t('admin.dashboard.rateLimits.loadFailed')
  } finally {
    if (accountController === controller) accountLoading.value = false
  }
}

async function loadKeys(): Promise<void> {
  keyController?.abort()
  const controller = new AbortController()
  keyController = controller
  keyLoading.value = true
  keyError.value = ''
  try {
    const response = await keysAPI.list(
      keyPage.value,
      keyPageSize,
      { search: keySearch.value.trim() || undefined, sort_by: 'id', sort_order: 'asc' },
      { signal: controller.signal }
    )
    if (keyController !== controller) return
    keyItems.value = response.items
    keyTotal.value = response.total
    keyLoaded.value = true
  } catch (error: any) {
    if (keyController !== controller) return
    if (error?.name === 'CanceledError' || error?.name === 'AbortError') return
    keyError.value = error?.message || t('admin.dashboard.rateLimits.loadFailed')
  } finally {
    if (keyController === controller) keyLoading.value = false
  }
}

function loadActiveTab(): Promise<void> {
  return activeTab.value === 'accounts' ? loadAccounts() : loadKeys()
}

function selectTab(tab: PanelTab): void {
  activeTab.value = tab
  liveMessage.value = ''
  if (tab === 'accounts' && !accountLoaded.value) void loadAccounts()
  if (tab === 'keys' && !keyLoaded.value) void loadKeys()
}

function handleSearch(): void {
  if (activeTab.value === 'accounts') accountPage.value = 1
  else keyPage.value = 1
  void loadActiveTab()
}

function changePage(page: number): void {
  if (activeTab.value === 'accounts') accountPage.value = page
  else keyPage.value = page
  void loadActiveTab()
}

async function refreshUpstream(): Promise<void> {
  const ids = refreshableAccountIds.value
  if (ids.length === 0 || liveRefreshing.value) return
  liveRefreshing.value = true
  liveMessage.value = ''
  try {
    const refreshed = await adminAPI.accounts.refreshUsageWindows(ids)
    const byId = new Map(refreshed.map((item) => [item.id, item]))
    accountItems.value = accountItems.value.map((item) => byId.get(item.id) ?? item)
    const failures = refreshed.filter((item) => item.refresh_error).length
    const successes = refreshed.length - failures
    liveMessage.value = failures > 0
      ? t('admin.dashboard.rateLimits.livePartial', { success: successes, failed: failures })
      : t('admin.dashboard.rateLimits.liveSuccess', { count: successes })
  } catch (error: any) {
    liveMessage.value = error?.message || t('admin.dashboard.rateLimits.liveFailed')
  } finally {
    liveRefreshing.value = false
  }
}

function formatUsd(value: number): string {
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

function formatKeyUsage(used: number, limit: number): string {
  if (limit <= 0) return t('admin.dashboard.rateLimits.unlimited')
  return `${formatUsd(used)} / ${formatUsd(limit)}`
}

function keyUtilization(used: number, limit: number): number | null {
  if (limit <= 0) return null
  return (used / limit) * 100
}

function compareKeyUsage(a: ApiKey, b: ApiKey): number {
  return keySortUtilization(b) - keySortUtilization(a)
    || (keyUtilization(b.usage_7d, b.rate_limit_7d) ?? -1) - (keyUtilization(a.usage_7d, a.rate_limit_7d) ?? -1)
    || (keyUtilization(b.usage_5h, b.rate_limit_5h) ?? -1) - (keyUtilization(a.usage_5h, a.rate_limit_5h) ?? -1)
    || a.name.localeCompare(b.name)
    || a.id - b.id
}

function keySortUtilization(key: ApiKey): number {
  return Math.max(
    keyUtilization(key.usage_5h, key.rate_limit_5h) ?? -1,
    keyUtilization(key.usage_7d, key.rate_limit_7d) ?? -1
  )
}

function isKeyActive(key: ApiKey): boolean {
  if (!key.last_used_at) return false
  const lastUsedAt = Date.parse(key.last_used_at)
  return Number.isFinite(lastUsedAt) && activityNow.value - lastUsedAt <= 5 * 60 * 1000
}

function clampedUtilization(value: number | null): number {
  return Math.min(100, Math.max(0, value ?? 0))
}

function utilizationClass(value: number | null): string {
  if ((value ?? 0) >= 90) return 'bg-red-500'
  if ((value ?? 0) >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}

function formatPercent(value: number | null): string {
  if (value === null) return '—'
  return `${value.toFixed(Number.isInteger(value) ? 0 : 1)}%`
}

function ownerLabel(key: ApiKey): string {
  return key.user?.username?.trim() || key.user?.email?.trim() || t('admin.dashboard.rateLimits.userId', { id: key.user_id })
}

function ownerTitle(key: ApiKey): string {
  if (!key.user) return ownerLabel(key)
  return [key.user.username, key.user.email].filter(Boolean).join(' / ')
}

function statusLabel(status: string): string {
  const key = `admin.dashboard.rateLimits.statuses.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

function statusClass(status: string): string {
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'error' || status === 'disabled' || status === 'quota_exhausted' || status === 'expired') {
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  }
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function refreshErrorLabel(errorCode: string): string {
  const key = `admin.dashboard.rateLimits.refreshErrors.${errorCode}`
  const translated = t(key)
  return translated === key ? t('admin.dashboard.rateLimits.liveFailed') : translated
}

onMounted(() => {
  activityNow.value = Date.now()
  activityTimer = setInterval(() => {
    activityNow.value = Date.now()
  }, 30_000)
  void loadKeys()
})
onBeforeUnmount(() => {
  accountController?.abort()
  keyController?.abort()
  if (activityTimer !== null) clearInterval(activityTimer)
})
</script>

<style scoped>
.key-row-cols {
  grid-template-columns: minmax(0, 1fr) auto;
}

.key-row-cols > :nth-child(3),
.key-row-cols > :nth-child(4) {
  grid-column: 1 / -1;
}

@media (min-width: 640px) {
  .key-row-cols {
    grid-template-columns: minmax(0, 1fr) auto;
  }
}

@media (min-width: 1024px) {
  .key-row-cols {
    grid-template-columns: minmax(0, 1fr) minmax(7.25rem, 0.9fr) minmax(7.25rem, 0.9fr);
  }

  .key-row-cols > :nth-child(2) {
    grid-column: 2 / -1;
    grid-row: 1;
  }

  .key-row-cols > :nth-child(3),
  .key-row-cols > :nth-child(4) {
    grid-column: auto;
  }
}
</style>
