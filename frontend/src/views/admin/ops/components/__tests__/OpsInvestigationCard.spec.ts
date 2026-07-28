import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import OpsInvestigationCard from '../OpsInvestigationCard.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const EmptyStateStub = defineComponent({
  name: 'EmptyState',
  template: '<div class="empty-state-stub" />'
})

const global = {
  stubs: { EmptyState: EmptyStateStub }
}

describe('OpsInvestigationCard', () => {
  it('renders evidence-backed findings and sends provider findings to upstream drill-down', async () => {
    const wrapper = mount(OpsInvestigationCard, {
      props: {
        loading: false,
        data: {
          start_time: '2026-07-12T10:00:00Z',
          end_time: '2026-07-12T11:00:00Z',
          baseline_start: '2026-07-12T03:00:00Z',
          baseline_end: '2026-07-12T10:00:00Z',
          total_errors: 5,
          findings: [
            {
              rule: 'provider_failure',
              kind: 'error',
              severity: 'warning',
              owner: 'provider',
              current_count: 5,
              baseline_count: 1,
              change_percent: 400,
              share_percent: 100
            }
          ]
        }
      },
      global
    })

    expect(wrapper.text()).toContain('admin.ops.investigation.rules.provider_failure.title')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('openErrorDetails')).toEqual([['upstream']])
  })

  it('renders the empty state when no finding reaches a rule threshold', () => {
    const wrapper = mount(OpsInvestigationCard, {
      props: {
        loading: false,
        data: {
          start_time: '2026-07-12T10:00:00Z',
          end_time: '2026-07-12T11:00:00Z',
          baseline_start: '2026-07-12T03:00:00Z',
          baseline_end: '2026-07-12T10:00:00Z',
          total_errors: 0,
          findings: []
        }
      },
      global
    })

    expect(wrapper.find('.empty-state-stub').exists()).toBe(true)
  })
})
