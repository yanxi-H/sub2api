import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { MonitorCenterOpenAIHistoryResponse, MonitorCenterProbeResponse } from '@/api/admin/monitorCenter'
import RealProbePanel from '../RealProbePanel.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

vi.mock('vue-chartjs', () => ({
  Line: { name: 'Line', props: ['data', 'options'], template: '<div class="line-chart" />' },
}))

function probe(points: MonitorCenterProbeResponse['points']): MonitorCenterProbeResponse {
  const latest = points[points.length - 1]
  return {
    configured: true,
    status: latest?.status ?? 'unknown',
    last_checked_at: latest?.timestamp,
    failure_reason: latest?.failure_reason,
    consecutive_failures: latest?.status === 'major_outage' ? 1 : 0,
    points,
  }
}

function history(points: MonitorCenterOpenAIHistoryResponse['points']): MonitorCenterOpenAIHistoryResponse {
  return {
    start_time: '2026-07-27T09:00:00Z',
    end_time: '2026-07-27T12:00:00Z',
    bucket: 'minute',
    points,
    incidents: [],
    statistics: { sample_count: points.length, successful_count: points.length, fetch_success_pct: 100, average_latency_ms: 100, anomaly_count: 0, groups: {} },
  }
}

describe('RealProbePanel', () => {
  it('does not call an empty selected range a local failure', () => {
    const wrapper = mount(RealProbePanel, {
      props: { loading: false, probe: probe([]), officialHistory: history([]) },
    })

    expect(wrapper.find('.mc-attribution').text()).toContain('admin.monitorCenter.probe.insufficientEvidence')
    expect(wrapper.find('.mc-attribution').text()).not.toContain('admin.monitorCenter.probe.localFirst')
  })

  it('correlates a failed probe with the closest official sample instead of the range latest sample', () => {
    const wrapper = mount(RealProbePanel, {
      props: {
        loading: false,
        probe: probe([{ timestamp: '2026-07-27T10:00:00Z', status: 'major_outage', failure_reason: 'timeout' }]),
        officialHistory: history([
          { timestamp: '2026-07-27T10:01:00Z', overall_status: 'degraded_performance', api_status: 'degraded_performance', chatgpt_status: 'operational', codex_status: 'operational', active_incident_count: 0, fetch_status: 'success', latency_ms: 100 },
          { timestamp: '2026-07-27T11:00:00Z', overall_status: 'operational', api_status: 'operational', chatgpt_status: 'operational', codex_status: 'operational', active_incident_count: 0, fetch_status: 'success', latency_ms: 100 },
        ]),
      },
    })

    expect(wrapper.find('.mc-attribution').text()).toContain('admin.monitorCenter.probe.suspectedUpstream')
  })

  it('uses the latest raw probe state instead of a worst-status chart bucket for attribution', () => {
    const value = probe([{ timestamp: '2026-07-27T10:00:00Z', status: 'major_outage', failure_reason: 'timeout' }])
    value.status = 'operational'
    value.last_checked_at = '2026-07-27T10:00:30Z'
    value.failure_reason = undefined
    value.consecutive_failures = 0

    const wrapper = mount(RealProbePanel, {
      props: {
        loading: false,
        probe: value,
        officialHistory: history([
          { timestamp: '2026-07-27T10:00:00Z', overall_status: 'operational', api_status: 'operational', chatgpt_status: 'operational', codex_status: 'operational', active_incident_count: 0, fetch_status: 'success', latency_ms: 100 },
        ]),
      },
    })

    expect(wrapper.find('.mc-attribution').text()).toContain('admin.monitorCenter.probe.pathAvailable')
  })
})
