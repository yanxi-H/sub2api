import { apiClient } from '../client'
import {
  opsAPI,
  type OpsDashboardOverview,
  type OpsErrorTrendResponse,
  type OpsLatencyTrendResponse,
  type OpsPerformanceDiagnosticsResponse,
  type OpsThroughputTrendResponse,
  type OpsUserConcurrencyTrendResponse,
} from './ops'

export type MonitorCenterStatus =
  | 'operational'
  | 'degraded_performance'
  | 'partial_outage'
  | 'major_outage'
  | 'under_maintenance'
  | 'unknown'

export interface MonitorCenterComponentStatus {
  key: string
  name: string
  status: MonitorCenterStatus
  matched: boolean
}

export interface MonitorCenterServiceGroup {
  key: 'api' | 'chatgpt' | 'codex' | string
  name: string
  status: MonitorCenterStatus
  components: MonitorCenterComponentStatus[]
}

export interface MonitorCenterIncidentUpdate {
  status: string
  body: string
  updated_at: string
}

export interface MonitorCenterIncident {
  id: string
  name: string
  status: string
  impact: string
  affected_components: string[]
  affected_groups: string[]
  started_at?: string | null
  created_at?: string | null
  updated_at: string
  resolved_at?: string | null
  url?: string
  latest_update?: MonitorCenterIncidentUpdate
  updates: MonitorCenterIncidentUpdate[]
}

export interface MonitorCenterOpenAIStatusResponse {
  overall_status: MonitorCenterStatus
  overall_description: string
  groups: MonitorCenterServiceGroup[]
  incidents: MonitorCenterIncident[]
  last_attempt_at?: string | null
  last_success_at?: string | null
  fetch_status: 'success' | 'failed' | string
  fetch_latency_ms: number
  stale: boolean
}

export interface MonitorCenterOpenAIHistoryPoint {
  timestamp: string
  overall_status: MonitorCenterStatus
  api_status: MonitorCenterStatus
  chatgpt_status: MonitorCenterStatus
  codex_status: MonitorCenterStatus
  active_incident_count: number
  fetch_status: 'success' | 'failed' | string
  latency_ms: number
  failure_reason?: string
  incident_refs?: Record<string, string[]>
}

export interface MonitorCenterOpenAIGroupStatistics {
  sample_count: number
  known_sample_count: number
  operational_count: number
  availability_pct: number
}

export interface MonitorCenterOpenAIHistoryStatistics {
  sample_count: number
  successful_count: number
  fetch_success_pct: number
  average_latency_ms: number
  anomaly_count: number
  groups: Record<string, MonitorCenterOpenAIGroupStatistics>
}

export interface MonitorCenterOpenAIHistoryResponse {
  start_time: string
  end_time: string
  bucket: string
  points: MonitorCenterOpenAIHistoryPoint[]
  statistics: MonitorCenterOpenAIHistoryStatistics
  incidents: MonitorCenterIncident[]
}

export interface MonitorCenterProbePoint {
  timestamp: string
  status: MonitorCenterStatus
  latency_ms?: number | null
  failure_reason?: string
}

export interface MonitorCenterProbeResponse {
  configured: boolean
  monitor_id?: number
  monitor_name?: string
  endpoint?: string
  model?: string
  endpoint_kind?: 'openai_direct' | 'custom_endpoint' | 'unknown' | string
  status: MonitorCenterStatus
  latency_ms?: number | null
  failure_reason?: string
  consecutive_failures: number
  last_checked_at?: string | null
  last_success_at?: string | null
  points: MonitorCenterProbePoint[]
}

export interface MonitorCenterRangeParams {
  time_range?: '1h' | '6h' | '24h'
  start_time?: string
  end_time?: string
}

export interface MonitorCenterHistoryParams extends Omit<MonitorCenterRangeParams, 'time_range'> {
  time_range?: MonitorCenterRangeParams['time_range'] | '3d'
}

export interface MonitorCenterRangeData {
  overview?: OpsDashboardOverview
  latency?: OpsLatencyTrendResponse
  concurrency?: OpsUserConcurrencyTrendResponse
  performance?: OpsPerformanceDiagnosticsResponse
  errors?: OpsErrorTrendResponse
  throughput?: OpsThroughputTrendResponse
}

export interface MonitorCenterThreeDayData {
  openai?: MonitorCenterOpenAIHistoryResponse
  probe?: MonitorCenterProbeResponse
  errors?: OpsErrorTrendResponse
  throughput?: OpsThroughputTrendResponse
}

export interface MonitorCenterBatchResult<T> {
  data: T
  success_count: number
  failure_count: number
}

async function getOpenAIStatus(signal?: AbortSignal): Promise<MonitorCenterOpenAIStatusResponse> {
  const { data } = await apiClient.get<MonitorCenterOpenAIStatusResponse>('/admin/monitor-center/openai/status', { signal })
  return data
}

async function getOpenAIHistory(
  params: MonitorCenterHistoryParams,
  signal?: AbortSignal,
): Promise<MonitorCenterOpenAIHistoryResponse> {
  const { data } = await apiClient.get<MonitorCenterOpenAIHistoryResponse>('/admin/monitor-center/openai/history', {
    params,
    signal,
  })
  return data
}

async function getProbe(params: MonitorCenterHistoryParams, signal?: AbortSignal): Promise<MonitorCenterProbeResponse> {
  const { data } = await apiClient.get<MonitorCenterProbeResponse>('/admin/monitor-center/probe', {
    params,
    signal,
  })
  return data
}

async function getRangeData(params: MonitorCenterRangeParams, signal?: AbortSignal): Promise<MonitorCenterBatchResult<MonitorCenterRangeData>> {
  const requestOptions = { signal }
  const results = await Promise.allSettled([
    opsAPI.getDashboardOverview(params, requestOptions),
    opsAPI.getLatencyTrend(params, requestOptions),
    opsAPI.getUserConcurrencyTrend(params, requestOptions),
    opsAPI.getPerformanceDiagnostics(params, requestOptions),
    opsAPI.getErrorTrend(params, requestOptions),
    opsAPI.getThroughputTrend(params, requestOptions),
  ])
  const keys = ['overview', 'latency', 'concurrency', 'performance', 'errors', 'throughput'] as const
  const data: MonitorCenterRangeData = {}
  let successCount = 0
  results.forEach((result, index) => {
    if (result.status === 'fulfilled') {
      Object.assign(data, { [keys[index]]: result.value })
      successCount += 1
    }
  })
  return { data, success_count: successCount, failure_count: results.length - successCount }
}

async function getThreeDayData(signal?: AbortSignal): Promise<MonitorCenterBatchResult<MonitorCenterThreeDayData>> {
  const end = new Date()
  const start = new Date(end.getTime() - 72 * 60 * 60 * 1000)
  const params = { start_time: start.toISOString(), end_time: end.toISOString() }
  const requestOptions = { signal }
  const results = await Promise.allSettled([
    getOpenAIHistory(params, signal),
    getProbe(params, signal),
    opsAPI.getErrorTrend(params, requestOptions),
    opsAPI.getThroughputTrend(params, requestOptions),
  ])
  const keys = ['openai', 'probe', 'errors', 'throughput'] as const
  const data: MonitorCenterThreeDayData = {}
  let successCount = 0
  results.forEach((result, index) => {
    if (result.status === 'fulfilled') {
      Object.assign(data, { [keys[index]]: result.value })
      successCount += 1
    }
  })
  return { data, success_count: successCount, failure_count: results.length - successCount }
}

export const monitorCenterAPI = {
  getOpenAIStatus,
  getOpenAIHistory,
  getProbe,
  getRangeData,
  getThreeDayData,
}

export type {
  OpsDashboardOverview,
  OpsErrorTrendResponse,
  OpsLatencyTrendResponse,
  OpsPerformanceDiagnosticsResponse,
  OpsThroughputTrendResponse,
  OpsUserConcurrencyTrendResponse,
}
