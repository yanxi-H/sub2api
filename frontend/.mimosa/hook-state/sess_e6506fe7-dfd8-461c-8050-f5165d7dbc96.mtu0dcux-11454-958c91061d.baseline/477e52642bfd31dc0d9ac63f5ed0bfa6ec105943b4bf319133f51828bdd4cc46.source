import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UpstreamStatusPanel from '../UpstreamStatusPanel.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('UpstreamStatusPanel', () => {
  it('keeps incidents separate from normal samples and only links affected groups', () => {
    const start = new Date('2026-07-26T00:00:00Z')
    const points = Array.from({ length: 15 }, (_, index) => ({
      timestamp: new Date(start.getTime() + (45 + index) * 60_000).toISOString(),
      overall_status: 'operational' as const,
      api_status: 'operational' as const,
      chatgpt_status: 'operational' as const,
      codex_status: 'operational' as const,
      active_incident_count: 1,
      fetch_status: 'success' as const,
      latency_ms: 200,
      incident_refs: index === 14 ? { all: ['incident-1'], api: ['incident-1'] } : {},
    }))

    const wrapper = mount(UpstreamStatusPanel, {
      props: {
        loading: false,
        rangeLabel: '1 hour',
        status: {
          overall_status: 'operational',
          overall_description: 'All Systems Operational',
          groups: [{
            key: 'api',
            name: 'API',
            status: 'operational',
            components: [{ key: 'responses', name: 'Responses', status: 'operational', matched: true }],
          }, {
            key: 'chatgpt',
            name: 'ChatGPT',
            status: 'operational',
            components: [{ key: 'login', name: 'Login', status: 'operational', matched: true }],
          }],
          incidents: [],
          fetch_status: 'success',
          fetch_latency_ms: 200,
          stale: false,
        },
        history: {
          start_time: start.toISOString(),
          end_time: new Date(start.getTime() + 60 * 60_000).toISOString(),
          bucket: 'minute',
          points,
          statistics: {
            sample_count: 15,
            successful_count: 15,
            fetch_success_pct: 100,
            average_latency_ms: 200,
            anomaly_count: 0,
            groups: {
              api: { sample_count: 15, known_sample_count: 15, operational_count: 15, availability_pct: 100 },
              chatgpt: { sample_count: 15, known_sample_count: 15, operational_count: 15, availability_pct: 100 },
            },
          },
          incidents: [],
        },
      },
    })

    const rows = wrapper.findAll('.mc-band-row')
    expect(rows).toHaveLength(2)
    expect(rows[0].findAll('.mc-band > i')).toHaveLength(36)
    expect(rows[0].findAll('.mc-band > i.linked')).toHaveLength(1)
    expect(rows[1].findAll('.mc-band > i.linked')).toHaveLength(0)
    expect(wrapper.findAll('.mc-band > i.incident')).toHaveLength(0)
  })
})
