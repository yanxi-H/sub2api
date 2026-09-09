import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UserDashboardModelRecommendations from '../UserDashboardModelRecommendations.vue'
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
  software_engineering_available: true,
  visual_spatial_available: true,
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
  ],
  software_engineering_recommendations: [
    { model: 'deepseek-v4-flash', effort: 'high', iq: 66, samples: 112, average_cost_usd: 0.22, average_cost_usd_by_band: { off_peak: 0.22, peak: 0.44 }, average_duration_minutes: 31 }
  ],
  visual_spatial_recommendations: [
    { model: 'gpt-5.6-sol', effort: 'high', iq: 97.5, samples: 86, average_cost_usd: 2.88, average_duration_minutes: 24.6 }
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

    const solEfforts = wrapper.findAll('.intelligence-rail-row[data-effort]').slice(0, 3).map((entry) => entry.attributes('data-effort'))
    expect(solEfforts).toEqual(['high', 'medium', 'low'])
    expect(wrapper.find('[data-best-combination="gpt-5.6-sol|medium"]').exists()).toBe(true)
  })

  it('defaults to score rails and keeps the decision metrics aligned', () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    expect(wrapper.find('[data-intelligence-mode="rail"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('[data-intelligence-groups]').classes()).toEqual(expect.arrayContaining(['md:grid-cols-2', 'xl:grid-cols-3']))
    expect(wrapper.find('.intelligence-rail-row').classes()).toContain('sm:grid-cols-[58px_minmax(7rem,15rem)_62px_90px]')
    expect(wrapper.findAll('.iq-track')).toHaveLength(4)
    expect(wrapper.findAll('.station-choice')).toHaveLength(2)
  })

  it('keeps reasoning effort names visible in their configured order', () => {
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

    expect(wrapper.findAll('.intelligence-rail-row[data-effort]').map((entry) => entry.attributes('data-effort'))).toEqual(['ultra', 'max'])
  })

  it('uses distinct low-saturation colors for each model tier', () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    expect(wrapper.find('[data-model="gpt-5.6-sol"]').attributes('style')).toContain('--model-color: #d5ad2d')
    expect(wrapper.find('[data-model="gpt-5.6-luna"]').attributes('style')).toContain('--model-color: #c4762b')
  })

  it('switches between score rail and compact matrix layouts', async () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    await wrapper.find('[data-intelligence-mode="matrix"]').trigger('click')

    expect(wrapper.find('[data-intelligence-mode="matrix"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.findAll('.intelligence-matrix-cell')).toHaveLength(4)
    expect(wrapper.find('.intelligence-rail-row').exists()).toBe(false)
  })

  it('switches capability dimensions and peak pricing from the latest metrics', async () => {
    const wrapper = mount(UserDashboardModelRecommendations, { props: { data } })

    await wrapper.find('[data-intelligence-dimension="software"]').trigger('click')
    expect(wrapper.find('[data-intelligence-dimension="software"]').attributes('aria-selected')).toBe('true')
    const softwareRow = wrapper.find('[data-combination="deepseek-v4-flash|high"]')
    expect(softwareRow.find('.model-iq').text()).toBe('66.0')
    expect(softwareRow.text()).toContain('$0.22')

    await wrapper.find('[data-price-band="peak"]').trigger('click')
    expect(wrapper.find('[data-price-band="peak"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('[data-combination="deepseek-v4-flash|high"]').text()).toContain('$0.44')

    await wrapper.find('[data-intelligence-dimension="visual"]').trigger('click')
    expect(wrapper.find('[data-intelligence-dimension="visual"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-combination="gpt-5.6-sol|high"] .model-iq').text()).toBe('97.5')
    expect(wrapper.find('[data-price-band="peak"]').exists()).toBe(false)
  })

  it('falls back to an available capability when comprehensive metrics are unavailable', () => {
    const wrapper = mount(UserDashboardModelRecommendations, {
      props: {
        data: {
          ...data,
          intelligence_available: false,
          intelligence_recommendations: []
        }
      }
    })

    expect(wrapper.find('[data-intelligence-dimension="comprehensive"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-intelligence-dimension="software"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-combination="deepseek-v4-flash|high"]').exists()).toBe(true)
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
