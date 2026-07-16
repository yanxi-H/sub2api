<template>
  <AppLayout>
    <TablePageLayout :title="t('admin.requestArchive.title')" :description="t('admin.requestArchive.description')">
      <!-- 搜索筛选栏 -->
      <template #toolbar>
        <div class="flex flex-wrap items-center gap-2">
          <SearchInput
            v-model="searchQuery"
            :placeholder="t('admin.requestArchive.searchPlaceholder')"
            @search="handleSearch"
          />
          <input
            v-model="dateFilter.start"
            type="date"
            class="input h-9 w-auto text-sm"
            :placeholder="t('admin.requestArchive.startDate')"
          />
          <input
            v-model="dateFilter.end"
            type="date"
            class="input h-9 w-auto text-sm"
            :placeholder="t('admin.requestArchive.endDate')"
          />
          <button class="btn btn-secondary h-9 text-sm" @click="handleSearch">
            <Icon name="search" size="sm" />
            {{ t('common.search') }}
          </button>
          <button class="btn btn-secondary h-9 w-9 p-0" :disabled="loading" @click="loadList">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <!-- 状态提示 -->
      <div v-if="!enabled" class="mb-4 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-400">
        {{ t('admin.requestArchive.disabledHint') }}
      </div>

      <!-- 表格 -->
      <DataTable :columns="columns" :data="items" :loading="loading">
        <template #cell-created_at="{ value }">
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(value) }}</span>
        </template>
        <template #cell-user_email="{ value, row }">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ value || `#${row.user_id}` }}</p>
            <p class="truncate text-xs text-gray-400">{{ row.api_key_name }}</p>
          </div>
        </template>
        <template #cell-model="{ value }">
          <span class="text-xs">{{ value }}</span>
        </template>
        <template #cell-prompt_preview="{ value, row }">
          <div class="flex max-w-md items-start gap-1">
            <span class="line-clamp-2 text-xs text-gray-600 dark:text-gray-300">{{ value }}</span>
            <span v-if="row.truncated" class="flex-shrink-0 text-[10px] text-amber-500">...</span>
          </div>
        </template>
        <template #cell-actions="{ row }">
          <button class="text-xs text-primary-600 hover:underline" @click="showDetail(row)">
            {{ t('common.view') }}
          </button>
        </template>
        <template #empty>
          <EmptyState :icon="'document'" :title="t('admin.requestArchive.empty')" />
        </template>
      </DataTable>

      <Pagination
        v-if="total > pageSize"
        :total="total"
        :page="page"
        :page-size="pageSize"
        :show-page-size-selector="false"
        @update:page="changePage"
      />

      <!-- 详情弹窗 -->
      <BaseDialog
        :show="detailDialog"
        :title="t('admin.requestArchive.detailTitle')"
        width="wide"
        @close="detailDialog = false"
      >
        <div v-if="detail" class="space-y-3">
          <div class="grid grid-cols-2 gap-3 text-sm">
            <div><span class="text-gray-500">{{ t('admin.requestArchive.time') }}:</span> {{ formatDateTime(detail.created_at) }}</div>
            <div><span class="text-gray-500">{{ t('admin.requestArchive.user') }}:</span> {{ detail.user_email || `#${detail.user_id}` }}</div>
            <div><span class="text-gray-500">{{ t('admin.requestArchive.model') }}:</span> {{ detail.model }}</div>
            <div><span class="text-gray-500">{{ t('admin.requestArchive.protocol') }}:</span> {{ detail.protocol }}</div>
            <div><span class="text-gray-500">{{ t('admin.requestArchive.endpoint') }}:</span> {{ detail.endpoint }}</div>
            <div><span class="text-gray-500">{{ t('admin.requestArchive.ip') }}:</span> {{ detail.ip_address }}</div>
          </div>
          <div>
            <p class="mb-1 text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.requestArchive.promptText') }}</p>
            <pre class="max-h-96 overflow-auto whitespace-pre-wrap rounded-lg bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-200">{{ detail.prompt_text }}</pre>
          </div>
        </div>
      </BaseDialog>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import requestArchiveAPI, { type RequestArchiveItem, type RequestArchiveDetail } from '@/api/admin/requestArchive'

const { t } = useI18n()
const items = ref<RequestArchiveItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const searchQuery = ref('')
const enabled = ref(false)
const detailDialog = ref(false)
const detail = ref<RequestArchiveDetail | null>(null)
const dateFilter = ref({ start: '', end: '' })

const columns = [
  { key: 'created_at', label: t('admin.requestArchive.time'), sortable: false },
  { key: 'user_email', label: t('admin.requestArchive.user'), sortable: false },
  { key: 'model', label: t('admin.requestArchive.model'), sortable: false },
  { key: 'prompt_preview', label: t('admin.requestArchive.prompt'), sortable: false },
  { key: 'actions', label: '', sortable: false }
]

async function loadList() {
  loading.value = true
  try {
    const resp = await requestArchiveAPI.list({
      search: searchQuery.value || undefined,
      start_date: dateFilter.value.start || undefined,
      end_date: dateFilter.value.end || undefined,
      page: page.value,
      page_size: pageSize.value
    })
    items.value = resp.items
    total.value = resp.total
  } catch {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function loadStatus() {
  try {
    const resp = await requestArchiveAPI.getStatus()
    enabled.value = resp.enabled
  } catch {
    enabled.value = false
  }
}

function handleSearch() {
  page.value = 1
  loadList()
}

function changePage(p: number) {
  page.value = p
  loadList()
}

async function showDetail(row: RequestArchiveItem) {
  try {
    detail.value = await requestArchiveAPI.getDetail(row.id)
    detailDialog.value = true
  } catch {
    // ignore
  }
}

onMounted(() => {
  loadStatus()
  loadList()
})
</script>
