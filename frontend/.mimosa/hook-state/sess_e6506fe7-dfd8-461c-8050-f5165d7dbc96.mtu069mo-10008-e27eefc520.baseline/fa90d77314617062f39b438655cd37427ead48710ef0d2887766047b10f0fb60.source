import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { MonitorCenterThreeDayData } from '@/api/admin/monitorCenter'
import ProbeHistoryPanel from '../ProbeHistoryPanel.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function data(): MonitorCenterThreeDayData {
  return {
    openai: {
      start_time: '2026-07-27T10:00:00Z',
      end_time: '2026-07-27T11:00:00Z',
      bucket: 'minute',
      points: [
        { timestamp: '2026-07-27T10:00:10Z', overall_status: 'major_outage', api_status: 'major_outage', chatgpt_status: 'operational', codex_status: 'operational', active_incident_count: 0, fetch_status: 'success', latency_ms: 100 },
        { timestamp: '2026-07-27T10:00:40Z', overall_status: 'unknown', api_status: 'operational', chatgpt_status: 'operational', codex_status: 'operational', active_incident_count: 0, fetch_status: 'failed', latency_ms: 500, failure_reason: 'timeout' },
      ],
      statistics: { sample_count: 2, successful_count: 1, fetch_success_pct: 50, average_latency_ms: 100, anomaly_count: 2, groups: {} },
      incidents: [],
    },
    throughput: { bucket: 'minute', points: [{ bucket_start: '2026-07-27T10:00:00Z', request_count: 10, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, requests_per_second: 0, tokens_per_second: 0 }] },
    errors: { bucket: 'minute', points: [{ bucket_start: '2026-07-27T10:00:00Z', error_count: 0, error_count_sla: 0, upstream_error_count: 0, business_limit_count: 0, error_rate: 0, upstream_error_rate: 0 }] },
    probe: { configured: true, status: 'major_outage', consecutive_failures: 1, points: [{ timestamp: '2026-07-27T10:00:00Z', status: 'major_outage', failure_reason: 'connection refused' }] },
  }
}

describe('ProbeHistoryPanel', () => {
  it('keeps the worst component status while marking a fetch failure in the same bucket', () => {
    const wrapper = mount(ProbeHistoryPanel, { props: { data: data(), rangeLabel: '1h', loading: false } })
    const firstBand = wrapper.findAll('.mc-history-row')[0]
    const populated = firstBand.findAll('.mc-history-band button').find(button => !button.classes().includes('empty'))

    expect(populated).toBeDefined()
    expect(populated?.attributes('style')).toContain('rgb(200, 77, 73)')
    expect(populated?.classes()).toContain('fetch-failed')
  })

  it('lists all five sources and filters anomalies across official, gateway, and probe bands', async () => {
    const wrapper = mount(ProbeHistoryPanel, { props: { data: data(), rangeLabel: '1h', loading: false } })
    expect(wrapper.findAll('.mc-history-record')).toHaveLength(5)

    await wrapper.get('.mc-history-list-head input').setValue(true)
    expect(wrapper.findAll('.mc-history-record')).toHaveLength(4)

    await wrapper.findAll('.mc-history-record')[0].trigger('click')
    expect(wrapper.find('.mc-sample-detail').exists()).toBe(true)
  })

  it('resets a stale selected sample when the global time range changes', async () => {
    const wrapper = mount(ProbeHistoryPanel, { props: { data: data(), rangeLabel: '1h', loading: false } })
    await wrapper.findAll('.mc-history-record')[0].trigger('click')

    const next = data()
    next.openai!.start_time = '2026-07-27T12:00:00Z'
    next.openai!.end_time = '2026-07-27T13:00:00Z'
    next.openai!.points = [{
      timestamp: '2026-07-27T12:30:00Z',
      overall_status: 'operational',
      api_status: 'operational',
      chatgpt_status: 'operational',
      codex_status: 'operational',
      active_incident_count: 0,
      fetch_status: 'success',
      latency_ms: 80,
    }]
    await wrapper.setProps({ data: next, rangeLabel: '6h' })

    expect(wrapper.find('.mc-sample-detail').text()).toContain('API')
    expect(wrapper.find('.mc-sample-detail').text()).toContain('80 ms')
    expect(wrapper.find('.mc-history-band button.selected').exists()).toBe(true)
  })
})
