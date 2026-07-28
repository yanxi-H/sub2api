import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import OpsUserConcurrencyTrendChart from '../OpsUserConcurrencyTrendChart.vue'

const { getUserConcurrencyTrend } = vi.hoisted(() => ({
  getUserConcurrencyTrend: vi.fn()
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: { getUserConcurrencyTrend }
}))

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  Legend: {},
  LineElement: {},
  LinearScale: {},
  PointElement: {},
  Tooltip: {}
}))

vi.mock('vue-chartjs', () => ({
  Line: defineComponent({
    name: 'LineChartStub',
    props: { data: { type: Object, required: true }, options: { type: Object, required: true } },
    template: '<div class="line-chart-stub" />'
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key
  })
}))

describe('OpsUserConcurrencyTrendChart', () => {
  it('shows normal, heavy, and recovery charts with shared user series', async () => {
    getUserConcurrencyTrend.mockResolvedValue({
      enabled: true,
      bucket: 'minute',
      current: { in_use: 7, waiting: 2, demand: 9 },
      current_lanes: {
        normal: { in_use: 4, waiting: 1, demand: 5 },
        heavy: { in_use: 2, waiting: 1, demand: 3 },
        recovery: { in_use: 1, waiting: 0, demand: 1 }
      },
      latency_lanes: {
        normal: { p95_ms: 950, p90_ms: 900, p50_ms: 500, avg_ms: 600, max_ms: 1200 },
        heavy: { p95_ms: 2500, p90_ms: 2200, p50_ms: 1500, avg_ms: 1700, max_ms: 3000 },
        recovery: { p95_ms: 4200, p90_ms: 4000, p50_ms: 3200, avg_ms: 3500, max_ms: 5000 }
      },
      points: [
        {
          bucket_start: '2026-07-19T12:00:00Z',
          system: { peak_in_use: 7, peak_waiting: 2, peak_demand: 9 },
          system_lanes: {
            normal: { peak_in_use: 4, peak_waiting: 1, peak_demand: 5 },
            heavy: { peak_in_use: 2, peak_waiting: 1, peak_demand: 3 },
            recovery: { peak_in_use: 1, peak_waiting: 0, peak_demand: 1 }
          },
          users: {
            '1': { peak_in_use: 4, peak_waiting: 1, peak_demand: 5 },
            '2': { peak_in_use: 2, peak_waiting: 0, peak_demand: 2 },
            '6': { peak_in_use: 1, peak_waiting: 1, peak_demand: 2 }
          },
          user_lanes: {
            '1': {
              normal: { peak_in_use: 2, peak_waiting: 1, peak_demand: 3 },
              heavy: { peak_in_use: 2, peak_waiting: 0, peak_demand: 2 },
              recovery: { peak_in_use: 0, peak_waiting: 0, peak_demand: 0 }
            },
            '2': {
              normal: { peak_in_use: 2, peak_waiting: 0, peak_demand: 2 },
              heavy: { peak_in_use: 0, peak_waiting: 0, peak_demand: 0 },
              recovery: { peak_in_use: 0, peak_waiting: 0, peak_demand: 0 }
            },
            '6': {
              normal: { peak_in_use: 0, peak_waiting: 0, peak_demand: 0 },
              heavy: { peak_in_use: 0, peak_waiting: 1, peak_demand: 1 },
              recovery: { peak_in_use: 1, peak_waiting: 0, peak_demand: 1 }
            }
          }
        }
      ],
      users: {
        '1': { user_id: 1, username: 'alpha', user_email: 'a@example.com', max_capacity: 5 },
        '2': { user_id: 2, username: 'beta', user_email: 'b@example.com', max_capacity: 3 },
        '6': { user_id: 6, username: 'zeta', user_email: 'z@example.com', max_capacity: 2 }
      }
    })

    const wrapper = mount(OpsUserConcurrencyTrendChart)
    await flushPromises()

    const charts = wrapper.findAllComponents({ name: 'LineChartStub' })
    expect(charts).toHaveLength(3)
    expect(charts[0].props('data').datasets.map((dataset: any) => dataset.label)).toEqual([
      'admin.ops.concurrencyTrend.systemActive',
      'admin.ops.concurrencyTrend.systemWaiting',
      'alpha',
      'beta'
    ])
    expect(charts[0].props('data').datasets[0].data).toEqual([4])
    expect(charts[1].props('data').datasets[0].data).toEqual([2])
    expect(charts[1].props('data').datasets[1].data).toEqual([1])
    expect(charts[2].props('data').datasets[0].data).toEqual([1])

    const normalLane = wrapper.find('[data-lane="normal"]')
    expect(normalLane.find('[data-metric="demand"][data-stat="p95"]').text()).toContain('5')
    expect(normalLane.find('[data-metric="demand"][data-stat="p90"]').text()).toContain('5')
    expect(normalLane.find('[data-metric="demand"][data-stat="p50"]').text()).toContain('5')
    expect(normalLane.find('[data-metric="demand"][data-stat="avg"]').text()).toContain('5')
    expect(normalLane.find('[data-metric="demand"][data-stat="max"]').text()).toContain('5')

    expect(wrapper.find('[data-lane="heavy"] [data-metric="demand"][data-stat="avg"]').text()).toContain('3')
    expect(wrapper.find('[data-lane="recovery"] [data-metric="demand"][data-stat="max"]').text()).toContain('1')
    expect(normalLane.find('[data-metric="latency"][data-stat="p95"]').text()).toContain('950 ms')
    expect(wrapper.find('[data-lane="heavy"] [data-metric="latency"][data-stat="avg"]').text()).toContain('1,700 ms')
    expect(wrapper.find('[data-lane="recovery"] [data-metric="latency"][data-stat="max"]').text()).toContain('5,000 ms')

    await wrapper.find('select').setValue('6')
    expect(wrapper.findAll('button').some(button => button.text().includes('zeta'))).toBe(true)
  })
})
