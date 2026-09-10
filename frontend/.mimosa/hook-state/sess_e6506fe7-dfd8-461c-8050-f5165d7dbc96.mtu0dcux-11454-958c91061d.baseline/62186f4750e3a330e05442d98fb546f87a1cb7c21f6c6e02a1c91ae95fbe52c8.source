import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import RequestLatencyChart from '../RequestLatencyChart.vue'

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

describe('RequestLatencyChart', () => {
  it('keeps E2E and TTFT as separate five-line series and preserves missing samples', async () => {
    const wrapper = mount(RequestLatencyChart, {
      props: {
        loading: false,
        range: '1h',
        overview: null,
        points: [
          {
            bucket_start: '2026-07-25T08:00:00Z',
            sample_count: 12,
            p95_ms: 950,
            p90_ms: 800,
            p50_ms: 420,
            avg_ms: 510,
            max_ms: 1800,
            ttft: { p95_ms: 740, p90_ms: 620, p50_ms: 260, avg_ms: 340, max_ms: 1400 }
          },
          {
            bucket_start: '2026-07-25T08:01:00Z',
            sample_count: 0,
            p95_ms: null,
            p90_ms: null,
            p50_ms: null,
            avg_ms: null,
            max_ms: null,
            ttft: { p95_ms: null, p90_ms: null, p50_ms: null, avg_ms: null, max_ms: null }
          }
        ]
      }
    })

    const e2eData = wrapper.getComponent({ name: 'Line' }).props('data') as any
    expect(e2eData.datasets.map((dataset: any) => dataset.label)).toEqual(['P95', 'P90', 'P50', 'Avg', 'Max'])
    expect(e2eData.datasets.map((dataset: any) => dataset.data)).toEqual([
      [950, null], [800, null], [420, null], [510, null], [1800, null]
    ])

    await wrapper.findAll('button')[1].trigger('click')
    const ttftData = wrapper.getComponent({ name: 'Line' }).props('data') as any
    expect(ttftData.datasets.map((dataset: any) => dataset.data)).toEqual([
      [740, null], [620, null], [260, null], [340, null], [1400, null]
    ])
  })
})
