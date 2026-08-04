import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UserDashboardModelRecommendations from '../UserDashboardModelRecommendations.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import type { CodexRadarDashboardRecommendations } from '@/api/usage'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, string>) => values?.time ? `${key} ${values.time}` : key
    })
  }
})

const data: CodexRadarDashboardRecommendations = {
  station_available: true,
  intelligence_available: true,
  station_recommendations: [
    {
      key: 'daily_development',
      title: 'Daily development',
      items: [
        { model: 'gpt-5.5', effort: 'xhigh', iq: 100.4, average_cost_usd: 5.75, average_duration_minutes: 23.5 },
        { model: 'gpt-5.6-luna', effort: 'max', iq: 96.4, average_cost_usd: 0.45, average_duration_minutes: 31.6 }
      ]
    }
  ],
  intelligence_recommendations: [
    { model: 'gpt-5.6-sol', effort: 'high', iq: 100, samples: 112, average_cost_usd: 10, average_duration_minutes: 50 },
    { model: 'gpt-5.6-sol', effort: 'low', iq: 60, samples: 112, average_cost_usd: 1, average_duration_minutes: 20 },
    { model: 'gpt-5.6-sol', effort: 'medium', iq: 90, samples: 112, average_cost_usd: 2, average_duration_minutes: 10 },
    { model: 'gpt-5.6-luna', effort: 'max', iq: 95, samples: 112, average_cost_usd: 1.5, average_duration_minutes: 15 }
  ]
}

describe('UserDashboardModelRecommendations', () => {
  it('keeps the station recommendation heading and limits the source rows to its supplied picks', () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    expect(wrapper.text()).toContain('dashboard.modelRecommendations.station')
    expect(wrapper.text()).toContain('5.5')
    expect(wrapper.text()).toContain('5.6 Luna Max')
  })

  it('groups models, sorts reasoning effort, and marks the best price-time-IQ balance', () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    const solEfforts = wrapper.findAll('.intelligence-card[data-effort]').slice(0, 3).map((entry) => entry.attributes('data-effort'))
    expect(solEfforts).toEqual(['high', 'medium', 'low'])
    expect(wrapper.find('[data-best-combination="gpt-5.6-sol|medium"]').exists()).toBe(true)
  })

  it('shows recommendation metrics as compact values instead of progress tracks', () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    expect(wrapper.find('.iq-profile-track').exists()).toBe(false)
    expect(wrapper.find('.signal-track').exists()).toBe(false)
    expect(wrapper.text()).toContain('dashboard.modelRecommendations.reasoningStrength')
    expect(wrapper.text()).toContain('dashboard.modelRecommendations.iqScore')
  })

  it('keeps reasoning effort names aligned with the full model variant name', () => {
    const wrapper = mount(UserDashboardModelRecommendations, {
      props: {
        data: {
          ...data,
          intelligence_recommendations: [
            { model: 'gpt-5.6-sol', effort: 'ultra', iq: 105.8, samples: 112, average_cost_usd: 21.89, average_duration_minutes: 53 },
            { model: 'gpt-5.6-sol', effort: 'max', iq: 107.1, samples: 112, average_cost_usd: 9.57, average_duration_minutes: 35 }
          ]
        }
      }
    })

    expect(wrapper.text()).toContain('5.6 Sol Ultra')
    expect(wrapper.text()).toContain('5.6 Sol Max')
  })

  it('uses the model provider mark with a color that represents reasoning strength', () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })
    const modelIcons = wrapper.findAllComponents(ModelIcon)

    expect(modelIcons.some((icon) => icon.props('model') === 'gpt-5.6-sol' && icon.props('color') === '#14b8a6')).toBe(true)
    expect(modelIcons.some((icon) => icon.props('model') === 'gpt-5.6-luna' && icon.props('color') === '#f59e0b')).toBe(true)
  })

  it('emits refresh from the icon button', async () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    await wrapper.find('[data-model-recommendations-refresh]').trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('emits refresh from the intelligence section action', async () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    await wrapper.find('[data-intelligence-recommendations-refresh]').trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })
})
