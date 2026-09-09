import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { OpsUserConcurrencyTrendResponse } from '@/api/admin/ops'
import ConcurrencyLanesChart from '../ConcurrencyLanesChart.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key }),
  }
})

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template: '<div class="line-chart" />',
  },
}))

function trendData(userCount = 8): OpsUserConcurrencyTrendResponse {
  const users = Object.fromEntries(Array.from({ length: userCount }, (_, index) => {
    const id = String(index + 1)
    return [id, { user_id: index + 1, user_email: `user${id}@example.com`, username: `User ${id}`, max_capacity: 10 }]
  }))
  const userLanes = Object.fromEntries(Array.from({ length: userCount }, (_, index) => {
    const demand = userCount - index
    return [String(index + 1), {
      normal: { peak_in_use: demand, peak_waiting: 1, peak_demand: demand + 1 },
      heavy: { peak_in_use: demand - 1, peak_waiting: 0, peak_demand: demand - 1 },
      recovery: { peak_in_use: 0, peak_waiting: index === 0 ? 1 : 0, peak_demand: index === 0 ? 1 : 0 },
    }]
  }))
  return {
    enabled: true,
    coverage_complete: true,
    bucket: 'minute',
    current: { in_use: 20, waiting: 2, demand: 22 },
    current_lanes: {
      normal: { in_use: 15, waiting: 2, demand: 17 },
      heavy: { in_use: 5, waiting: 0, demand: 5 },
      recovery: { in_use: 0, waiting: 0, demand: 0 },
    },
    points: [{
      bucket_start: '2026-07-27T08:00:00Z',
      system: { peak_in_use: 20, peak_waiting: 2, peak_demand: 22 },
      users: {},
      system_lanes: {
        normal: { peak_in_use: 15, peak_waiting: 2, peak_demand: 17 },
        heavy: { peak_in_use: 5, peak_waiting: 0, peak_demand: 5 },
        recovery: { peak_in_use: 0, peak_waiting: 1, peak_demand: 1 },
      },
      user_lanes: userLanes,
    }],
    users,
  }
}

describe('ConcurrencyLanesChart', () => {
  it('shows Top N users and an aggregate remainder by default while retaining system series', () => {
    const wrapper = mount(ConcurrencyLanesChart, { props: { data: trendData(), loading: false } })
    const normal = wrapper.findAllComponents({ name: 'Line' })[0].props('data') as any

    expect(normal.datasets.slice(0, 2).map((dataset: any) => dataset.metric)).toEqual(['system', 'queue'])
    expect(normal.datasets.filter((dataset: any) => dataset.metric === 'user')).toHaveLength(6)
    const other = normal.datasets.find((dataset: any) => dataset.metric === 'other')
    expect(other.data).toEqual([3])
    expect(new Set(normal.datasets.filter((dataset: any) => dataset.metric === 'user').map((dataset: any) => dataset.borderColor)).size).toBe(6)
  })

  it('focuses one user across all lanes and provides an explicit return to all users', async () => {
    const wrapper = mount(ConcurrencyLanesChart, { props: { data: trendData(), loading: false } })
    await wrapper.get('select').setValue('2')

    for (const chart of wrapper.findAllComponents({ name: 'Line' })) {
      const datasets = (chart.props('data') as any).datasets
      expect(datasets.filter((dataset: any) => dataset.metric === 'user').map((dataset: any) => dataset.label)).toEqual(['User 2'])
      expect(datasets.some((dataset: any) => dataset.metric === 'other')).toBe(false)
    }

    const back = wrapper.get('.mc-all-users-button')
    await back.trigger('click')
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('')
  })

  it('reports demand, active, and queued values for a user at the precise sample time', () => {
    const wrapper = mount(ConcurrencyLanesChart, { props: { data: trendData(1), loading: false } })
    const chart = wrapper.findAllComponents({ name: 'Line' })[0]
    const options = chart.props('options') as any
    const data = chart.props('data') as any
    const userDataset = data.datasets.find((dataset: any) => dataset.metric === 'user')

    expect(options.plugins.tooltip.callbacks.title([{ dataIndex: 0 }])).not.toBe('')
    expect(options.plugins.tooltip.callbacks.label({ dataset: userDataset, dataIndex: 0, parsed: { y: 9 } }))
      .toContain('"demand":2')
    expect(options.plugins.tooltip.callbacks.label({ dataset: userDataset, dataIndex: 0, parsed: { y: 2 } }))
      .toContain('"waiting":1')
  })
})
