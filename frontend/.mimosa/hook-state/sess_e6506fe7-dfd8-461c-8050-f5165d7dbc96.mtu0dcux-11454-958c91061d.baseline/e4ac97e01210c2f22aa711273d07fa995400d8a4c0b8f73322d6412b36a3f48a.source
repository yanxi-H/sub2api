import { afterEach, describe, expect, it, vi } from 'vitest'

import { monitorCenterAPI } from '../monitorCenter'
import { opsAPI } from '../ops'
import { apiClient } from '../../client'

describe('monitorCenterAPI.getRangeData', () => {
  afterEach(() => vi.restoreAllMocks())

  it('returns successful modules when one ops request fails', async () => {
    vi.spyOn(opsAPI, 'getDashboardOverview').mockResolvedValue({ health_score: 92 } as never)
    vi.spyOn(opsAPI, 'getLatencyTrend').mockRejectedValue(new Error('latency unavailable'))
    vi.spyOn(opsAPI, 'getUserConcurrencyTrend').mockResolvedValue({ points: [] } as never)
    vi.spyOn(opsAPI, 'getPerformanceDiagnostics').mockResolvedValue({ trend: [] } as never)
    vi.spyOn(opsAPI, 'getErrorTrend').mockResolvedValue({ points: [] } as never)
    vi.spyOn(opsAPI, 'getThroughputTrend').mockResolvedValue({ points: [] } as never)

    const result = await monitorCenterAPI.getRangeData({ time_range: '1h' })

    expect(result.success_count).toBe(5)
    expect(result.failure_count).toBe(1)
    expect(result.data.overview).toEqual({ health_score: 92 })
    expect(result.data.latency).toBeUndefined()
    expect(result.data.concurrency).toEqual({ points: [] })
  })
})

describe('monitorCenterAPI range requests', () => {
  afterEach(() => vi.restoreAllMocks())

  it('passes custom start and end times to OpenAI history and real probe endpoints', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { points: [] } })
    const params = { start_time: '2026-07-25T00:00:00Z', end_time: '2026-07-26T00:00:00Z' }

    await monitorCenterAPI.getOpenAIHistory(params)
    await monitorCenterAPI.getProbe(params)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/monitor-center/openai/history', expect.objectContaining({ params }))
    expect(get).toHaveBeenNthCalledWith(2, '/admin/monitor-center/probe', expect.objectContaining({ params }))
  })
})
