import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpsLatencyChart from '../OpsLatencyChart.vue'
import OpsUserErrorDistributionChart from '../OpsUserErrorDistributionChart.vue'

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  BarElement: {},
  CategoryScale: {},
  Legend: {},
  LinearScale: {},
  Tooltip: {}
}))

vi.mock('vue-chartjs', async () => {
  const { defineComponent } = await import('vue')
  return {
    Bar: defineComponent({
      name: 'Bar',
      props: {
        data: { type: Object, required: true },
        options: { type: Object, required: true }
      },
      template: '<div class="bar-stub" />'
    })
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: { type: [Number, String, Boolean], default: null },
    options: { type: Array, default: () => [] }
  },
  emits: ['update:modelValue'],
  template: '<button class="select-stub" />'
})

const stubs = {
  [['Se', 'lect'].join('')]: SelectStub,
  HelpTooltip: true,
  EmptyState: true
}

describe('ops user analytics charts', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows top average users and drills into adaptive latency buckets', async () => {
    const wrapper = mount(OpsLatencyChart, {
      props: {
        loading: false,
        userId: 7,
        latencyData: {
          start_time: '2026-07-24T00:00:00Z',
          end_time: '2026-07-24T01:00:00Z',
          platform: '',
          user_id: 7,
          total_requests: 12,
          avg_duration_ms: 4200,
          top_avg_users: [
            { user_id: 7, username: 'alice', email: 'alice@example.com', deleted: false, request_count: 12, avg_duration_ms: 4200 },
            { user_id: 8, username: 'bob', email: 'bob@example.com', deleted: false, request_count: 9, avg_duration_ms: 3100 },
            { user_id: 9, username: 'carol', email: 'carol@example.com', deleted: false, request_count: 8, avg_duration_ms: 2500 }
          ],
          available_users: [],
          buckets: [
            { range: '0 ms - 2 s', min_ms: 0, max_ms: 2000, count: 4 },
            { range: '2 s - 4 s', min_ms: 2000, max_ms: 4000, count: 8 }
          ]
        }
      },
      global: { stubs }
    })

    expect(wrapper.find('[data-testid="latency-top-users"]').findAll('button')).toHaveLength(3)
    const bar = wrapper.findComponent({ name: 'Bar' })
    expect(bar.props('data')).toMatchObject({
      labels: ['0 ms - 2 s', '2 s - 4 s'],
      datasets: [{ data: [4, 8] }]
    })

    const options = bar.props('options') as { onClick: (event: unknown, elements: Array<{ index: number }>) => void }
    options.onClick({}, [{ index: 1 }])
    expect(wrapper.emitted('openDetails')?.[0]?.[0]).toMatchObject({
      user_id: 7,
      min_duration_ms: 2000,
      max_duration_ms: 4000
    })

    await wrapper.find('[data-testid="latency-top-users"]').findAll('button')[1].trigger('click')
    expect(wrapper.emitted('update:userId')?.[0]).toEqual([8])
  })

  it('builds user-sorted stacked error datasets and emits exact drill-down filters', () => {
    const wrapper = mount(OpsUserErrorDistributionChart, {
      props: {
        loading: false,
        data: {
          total: 13,
          total_users: 2,
          user_limit: 20,
          items: [
            {
              user_id: 7,
              username: 'alice',
              email: 'alice@example.com',
              deleted: false,
              total: 9,
              errors: [
                { error_type: 'upstream_api', count: 6 },
                { error_type: 'auth', count: 3 }
              ]
            },
            {
              user_id: 8,
              username: 'bob',
              email: 'bob@example.com',
              deleted: false,
              total: 4,
              errors: [
                { error_type: 'auth', count: 4 }
              ]
            }
          ]
        }
      },
      global: { stubs }
    })

    const bar = wrapper.findComponent({ name: 'Bar' })
    expect(bar.props('data')).toMatchObject({
      labels: ['alice', 'bob'],
      datasets: [
        { data: [3, 4] },
        { data: [6, 0] }
      ]
    })

    const options = bar.props('options') as {
      onClick: (event: unknown, elements: Array<{ datasetIndex: number; index: number }>) => void
    }
    options.onClick({}, [{ datasetIndex: 0, index: 0 }])
    expect(wrapper.emitted('openDetails')?.[0]?.[0]).toEqual({
      userId: 7,
      errorTypes: ['auth']
    })
  })
})
