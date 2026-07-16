/**
 * Request Archive API - 请求文本存档(风控筛查)
 */
import { apiClient } from '../client'

export interface RequestArchiveItem {
  id: number
  created_at: string
  request_id: string
  user_id: number
  user_email: string
  api_key_id: number
  api_key_name: string
  group_id: number | null
  endpoint: string
  protocol: string
  model: string
  ip_address: string
  prompt_preview: string
  truncated: boolean
}

export interface RequestArchiveDetail extends RequestArchiveItem {
  prompt_text: string
}

export interface RequestArchiveListResponse {
  items: RequestArchiveItem[]
  total: number
  page: number
  page_size: number
}

export interface RequestArchiveQuery {
  search?: string
  user_id?: number
  api_key_id?: number
  start_date?: string
  end_date?: string
  page?: number
  page_size?: number
}

export async function list(params: RequestArchiveQuery = {}): Promise<RequestArchiveListResponse> {
  const { data } = await apiClient.get<RequestArchiveListResponse>('/admin/request-archive', { params })
  return data
}

export async function getDetail(id: number): Promise<RequestArchiveDetail> {
  const { data } = await apiClient.get<RequestArchiveDetail>(`/admin/request-archive/${id}`)
  return data
}

export async function getStatus(): Promise<{ enabled: boolean }> {
  const { data } = await apiClient.get<{ enabled: boolean }>('/admin/request-archive/status')
  return data
}

export const requestArchiveAPI = { list, getDetail, getStatus }
export default requestArchiveAPI
