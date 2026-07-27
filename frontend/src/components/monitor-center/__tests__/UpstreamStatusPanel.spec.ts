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
  it('renders fixed real-time slots, keeps missing coverage gray, and marks incidents', () => {
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
        },
      },
    })

    const slots = wrapper.findAll('.mc-band > i')
    expect(slots).toHaveLength(30)
    expect(slots.filter(slot => slot.classes().includes('missing'))).toHaveLength(22)
    expect(slots.filter(slot => slot.classes().includes('incident'))).toHaveLength(8)
  })
})
