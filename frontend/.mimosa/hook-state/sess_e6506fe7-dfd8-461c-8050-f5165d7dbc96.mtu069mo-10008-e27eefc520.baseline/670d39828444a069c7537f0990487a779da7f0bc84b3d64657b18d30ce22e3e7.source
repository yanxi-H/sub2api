import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { OpsPerformanceImpact } from '@/api/admin/ops'
import SlowImpactTable from '../SlowImpactTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function impact(overrides: Partial<OpsPerformanceImpact>): OpsPerformanceImpact {
  return {
    dimension: 'user',
    id: '1',
    name: 'User 1',
    request_count: 1,
    slow_rate: 1,
    e2e_p95_ms: 100,
    ttft_p95_ms: 50,
    queue_p95_ms: 10,
    main_cause: 'healthy',
    ...overrides,
  }
}

function renderedNames(wrapper: ReturnType<typeof mount>): string[] {
  return wrapper.findAll('tbody tr').map((row) => row.find('td strong').text())
}

describe('SlowImpactTable', () => {
  it('renders every impact in the selected dimension instead of truncating the list', () => {
    const users = Array.from({ length: 12 }, (_, index) => impact({
      id: String(index + 1),
      name: `User ${index + 1}`,
    }))
    const wrapper = mount(SlowImpactTable, {
      props: { impacts: [...users, impact({ dimension: 'account', id: 'a1', name: 'Account 1' })] },
    })

    expect(wrapper.findAll('tbody tr')).toHaveLength(12)
    expect(renderedNames(wrapper)).toContain('User 12')
    expect(wrapper.find('.mc-table-wrap').classes()).toContain('mc-table-wrap')
  })

  it('sorts numeric columns ascending and descending while keeping missing values last', async () => {
    const wrapper = mount(SlowImpactTable, {
      props: {
        impacts: [
          impact({ id: 'a', name: 'Alpha', e2e_p95_ms: 300 }),
          impact({ id: 'b', name: 'Beta', e2e_p95_ms: null }),
          impact({ id: 'c', name: 'Gamma', e2e_p95_ms: 100 }),
        ],
      },
    })
    const header = wrapper.get('[data-sort-key="e2e_p95_ms"]')

    await header.trigger('click')
    expect(renderedNames(wrapper)).toEqual(['Gamma', 'Alpha', 'Beta'])
    expect(header.element.closest('th')?.getAttribute('aria-sort')).toBe('ascending')

    await header.trigger('click')
    expect(renderedNames(wrapper)).toEqual(['Alpha', 'Gamma', 'Beta'])
    expect(header.element.closest('th')?.getAttribute('aria-sort')).toBe('descending')
  })

  it('sorts text columns and switches cleanly between users, accounts, and models', async () => {
    const wrapper = mount(SlowImpactTable, {
      props: {
        impacts: [
          impact({ id: '2', name: 'Zulu' }),
          impact({ id: '1', name: 'Alpha' }),
          impact({ dimension: 'account', id: 'a1', name: 'Account Z' }),
          impact({ dimension: 'account', id: 'a2', name: 'Account A' }),
          impact({ dimension: 'model', id: 'm1', name: 'gpt-5' }),
        ],
      },
    })

    await wrapper.get('[data-sort-key="name"]').trigger('click')
    expect(renderedNames(wrapper)).toEqual(['Alpha', 'Zulu'])

    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    expect(renderedNames(wrapper)).toEqual(['Account A', 'Account Z'])

    await wrapper.findAll('[role="tab"]')[2].trigger('click')
    expect(renderedNames(wrapper)).toEqual(['gpt-5'])
  })
})
