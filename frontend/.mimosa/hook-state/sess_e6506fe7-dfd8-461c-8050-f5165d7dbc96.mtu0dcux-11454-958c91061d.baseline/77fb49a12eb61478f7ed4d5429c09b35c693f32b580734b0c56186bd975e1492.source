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

export interface BatchAPIKeyResult {
  items: ApiKey[]
  updated_count: number
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
  batchSync7dWindow,
  batchReset7dUsage
}

export default apiKeysAPI
