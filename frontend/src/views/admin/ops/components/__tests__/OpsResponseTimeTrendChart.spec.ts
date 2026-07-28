import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import OpsResponseTimeTrendChart from '../OpsResponseTimeTrendChart.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template: '<div class="line-chart" />'
  }
}))

describe('OpsResponseTimeTrendChart', () => {
  it('renders P95, P90, P50, Avg, and Max in the requested order without zero-filling gaps', () => {
    const wrapper = mount(OpsResponseTimeTrendChart, {
      props: {
        loading: false,
        timeRange: '1h',
        points: [
          {
            bucket_start: '2026-07-25T08:00:00Z',
            sample_count: 12,
            p95_ms: 950,
            p90_ms: 800,
            p50_ms: 420,
            avg_ms: 510,
            max_ms: 1800
          },
          {
            bucket_start: '2026-07-25T08:01:00Z',
            sample_count: 0,
            p95_ms: null,
            p90_ms: null,
            p50_ms: null,
            avg_ms: null,
            max_ms: null
          }
        ]
      },
      global: { stubs: { HelpTooltip: true, EmptyState: true } }
    })

    const data = wrapper.getComponent({ name: 'Line' }).props('data') as any
    expect(data.datasets.map((dataset: any) => dataset.label)).toEqual(['P95', 'P90', 'P50', 'Avg', 'Max'])
    expect(data.datasets.map((dataset: any) => dataset.data)).toEqual([
      [950, null],
      [800, null],
      [420, null],
      [510, null],
      [1800, null]
    ])
  })
})
