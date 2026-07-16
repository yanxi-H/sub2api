/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey } from '@/types'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(id: number, groupId: number | null): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    group_id: groupId === null ? 0 : groupId
  })
  return data
}

export interface WindowStartOverride {
  window_5h_start?: string // RFC3339;对齐 5h 窗口起始时刻（保留 usage）
  window_1d_start?: string // RFC3339;对齐 1d 窗口起始时刻（保留 usage）
  window_7d_start?: string // RFC3339;对齐 7d 窗口起始时刻（保留 usage）
}

/**
 * 管理员：仅调整速率限制窗口的起始时间（保留 usage 已用金额）。
 * 用于把 sub2api Key 的窗口起点对齐到 Codex 官方账号的真实刷新周期，
 * 使 /v1/usage 返回的 reset_at 与官方账号一致。
 */
export async function setApiKeyWindowStart(id: number, override: WindowStartOverride): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, override)
  return data
}

export interface BatchAPIKeyResult {
  items: ApiKey[]
  updated_count: number
}

export async function batchSync7dWindow(apiKeyIds: number[], groupId: number | null, accountId: number): Promise<BatchAPIKeyResult> {
  const { data } = await apiClient.post<BatchAPIKeyResult>('/admin/api-keys/batch-sync-7d-window', {
    api_key_ids: apiKeyIds,
    group_id: groupId ?? 0,
    account_id: accountId
  })
  return data
}

export async function batchReset7dUsage(apiKeyIds: number[], groupId: number | null): Promise<BatchAPIKeyResult> {
  const { data } = await apiClient.post<BatchAPIKeyResult>('/admin/api-keys/batch-reset-7d-usage', {
    api_key_ids: apiKeyIds,
    group_id: groupId ?? 0
  })
  return data
}

export const apiKeysAPI = {
  updateApiKeyGroup,
  setApiKeyWindowStart,
  batchSync7dWindow,
  batchReset7dUsage
}

export default apiKeysAPI
