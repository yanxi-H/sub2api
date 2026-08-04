import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DashboardView from '../DashboardView.vue'

const mocks = vi.hoisted(() => ({
  refreshUser: vi.fn(),
  getDashboardStats: vi.fn(),
  getDashboardRecommendations: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  getMyPlatformQuotas: vi.fn()
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 0 },
    isSimpleMode: false,
    refreshUser: mocks.refreshUser
  })
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats: mocks.getDashboardStats,
    getDashboardRecommendations: mocks.getDashboardRecommendations,
    getDashboardTrend: mocks.getDashboardTrend,
    getDashboardModels: mocks.getDashboardModels,
    getByDateRange: mocks.getByDateRange
  }
}))

vi.mock('@/api/user', () => ({
  getMyPlatformQuotas: mocks.getMyPlatformQuotas
}))

const dashboardStats = {
  total_api_keys: 0,
  active_api_keys: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  average_duration_ms: 0,
  rpm: 0,
  tpm: 0
}

const recommendationData = {
  station_available: true,
  intelligence_available: true,
  station_recommendations: [],
  intelligence_recommendations: []
}

function mountDashboard() {
  return mount(DashboardView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: true,
        UserDashboardStats: true,
        UserDashboardCharts: true,
        UserDashboardRecentUsage: true,
        UserDashboardQuickActions: true,
        UserDashboardModelRecommendations: {
          emits: ['refresh'],
          template: '<button data-recommendations-refresh @click="$emit(\'refresh\')">refresh</button>'
        }
      }
    }
  })
}

describe('DashboardView model recommendations', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.refreshUser.mockResolvedValue(undefined)
    mocks.getDashboardStats.mockResolvedValue(dashboardStats)
    mocks.getDashboardRecommendations.mockResolvedValue(recommendationData)
    mocks.getDashboardTrend.mockResolvedValue({ trend: [] })
    mocks.getDashboardModels.mockResolvedValue({ models: [] })
    mocks.getByDateRange.mockResolvedValue({ items: [] })
    mocks.getMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
  })

  it('loads recommendations on page entry and reloads them from the panel refresh control', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    expect(mocks.getDashboardRecommendations).toHaveBeenCalledTimes(1)

    await wrapper.find('[data-recommendations-refresh]').trigger('click')
    await flushPromises()

    expect(mocks.getDashboardRecommendations).toHaveBeenCalledTimes(2)
  })
})
