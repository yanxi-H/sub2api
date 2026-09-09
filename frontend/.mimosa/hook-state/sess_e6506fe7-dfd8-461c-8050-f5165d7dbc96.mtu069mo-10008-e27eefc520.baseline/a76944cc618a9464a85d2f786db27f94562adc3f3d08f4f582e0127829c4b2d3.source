import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

import RecentTimeRange from '../RecentTimeRange.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, number>) => {
      if (key === 'usage.recentMinutes') return `Last ${params?.count}m`
      if (key === 'usage.recentHours') return `Last ${params?.count}h`
      if (key === 'usage.recentHalfHour') return 'Last 30m'
      return key
    },
    locale: ref('en-US'),
  }),
}))

describe('RecentTimeRange', () => {
  it('shows all shortcuts and emits the selected duration', async () => {
    const wrapper = mount(RecentTimeRange, {
      props: { activeMinutes: 5 },
    })

    expect(wrapper.text()).toContain('Last 1m')
    expect(wrapper.text()).toContain('Last 5m')
    expect(wrapper.text()).toContain('Last 10m')
    expect(wrapper.text()).toContain('Last 30m')
    expect(wrapper.text()).toContain('Last 3h')
    expect(wrapper.text()).toContain('Last 6h')
    expect(wrapper.find('[aria-pressed="true"]').text()).toBe('Last 5m')

    await wrapper.findAll('button')[4].trigger('click')
    expect(wrapper.emitted('select')).toEqual([[180]])
  })

  it('displays the exact selected timestamps', () => {
    const wrapper = mount(RecentTimeRange, {
      props: {
        activeMinutes: 1,
        startTime: '2026-08-24T02:00:00.000Z',
        endTime: '2026-08-24T02:01:00.000Z',
      },
    })

    expect(wrapper.get('[data-testid="exact-time-range"]').text()).toMatch(
      /\d{2}:\d{2}:\d{2}.*-.*\d{2}:\d{2}:\d{2}/
    )
  })
})
