import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { ApiKey } from '@/types'
import RateLimitOverviewPanel from '../RateLimitOverviewPanel.vue'

const { listUsageWindows, refreshUsageWindows, listAccounts, listKeys, batchSync7dWindow, batchReset7dUsage, showSuccess, showError } = vi.hoisted(() => ({
  listUsageWindows: vi.fn(),
  refreshUsageWindows: vi.fn(),
  listAccounts: vi.fn(),
  listKeys: vi.fn(),
  batchSync7dWindow: vi.fn(),
  batchReset7dUsage: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      listUsageWindows,
      refreshUsageWindows,
      list: listAccounts
    },
    apiKeys: { batchSync7dWindow, batchReset7dUsage }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('@/api/keys', () => ({
  default: { list: listKeys }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key} ${Object.values(params).join(' ')}`
      }
    })
  }
})

const accountItem = {
  id: 11,
  name: 'Codex Primary',
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  five_hour: { utilization: 35, resets_at: null, remaining_seconds: 0 },
  seven_day: { utilization: 68, resets_at: null, remaining_seconds: 0 },
  updated_at: null,
  supports_live_refresh: true
}

const apiKey = {
  id: 21,
  user_id: 7,
  name: 'Production Key',
  status: 'active',
  rate_limit_5h: 25,
  rate_limit_7d: 100,
  usage_5h: 5,
  usage_7d: 40,
  reset_5h_at: null,
  reset_7d_at: null,
  last_used_at: new Date().toISOString(),
  group_id: 3,
  group: { id: 3, name: 'Production', platform: 'openai' },
  user: { id: 7, username: 'yuan', email: 'yuan@example.com', role: 'user', status: 'active' }
} as ApiKey

const higherUsageApiKey = {
  ...apiKey,
  id: 23,
  name: 'High Usage Key',
  usage_5h: 10,
  usage_7d: 70
} as ApiKey

const otherOwnerApiKey = {
  ...apiKey,
  id: 22,
  user_id: 8,
  name: 'Team Key',
  usage_5h: 15,
  usage_7d: 60,
  group_id: 4,
  group: { id: 4, name: 'Staging', platform: 'openai' },
  user: { id: 8, username: 'alice', email: 'alice@example.com', role: 'user', status: 'active' }
} as ApiKey

describe('RateLimitOverviewPanel', () => {
  beforeEach(() => {
    listUsageWindows.mockReset()
    refreshUsageWindows.mockReset()
    listAccounts.mockReset()
    listKeys.mockReset()
    batchSync7dWindow.mockReset()
    batchReset7dUsage.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    listUsageWindows.mockResolvedValue({ items: [accountItem], total: 1, page: 1, page_size: 10, pages: 1 })
    listKeys.mockResolvedValue({
      items: [apiKey, otherOwnerApiKey, higherUsageApiKey],
      total: 3,
      page: 1,
      page_size: 10,
      pages: 1
    })
    refreshUsageWindows.mockResolvedValue([
      {
        ...accountItem,
        five_hour: { utilization: 91, resets_at: null, remaining_seconds: 0 }
      }
    ])
    listAccounts.mockResolvedValue({
      items: [{
        id: 31,
        name: 'Production Upstream',
        platform: 'openai',
        extra: { codex_7d_reset_at: new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString() }
      }],
      total: 1,
      page: 1,
      page_size: 1000,
      pages: 1
    })
    batchSync7dWindow.mockResolvedValue({ items: [], updated_count: 1 })
    batchReset7dUsage.mockResolvedValue({ items: [], updated_count: 1 })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows account windows and refreshes supported accounts from upstream', async () => {
    const wrapper = mount(RateLimitOverviewPanel)
    await flushPromises()

    await wrapper.get('[data-testid="accounts-tab"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Codex Primary')
    expect(wrapper.text()).toContain('35%')
    expect(wrapper.text()).toContain('68%')

    await wrapper.get('[data-testid="live-refresh"]').trigger('click')
    await flushPromises()

    expect(refreshUsageWindows).toHaveBeenCalledWith([11])
    expect(wrapper.text()).toContain('91%')
  })

  it('groups API keys by assigned group, sorts keys by utilization, and marks recent activity', async () => {
    const wrapper = mount(RateLimitOverviewPanel)
    await flushPromises()

    expect(listKeys).toHaveBeenCalledWith(
      1,
      1000,
      expect.objectContaining({ sort_by: 'id', sort_order: 'asc' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )

    const groups = wrapper.findAll('[data-testid="key-group"]')
    expect(groups).toHaveLength(2)
    expect(groups[0].get('[data-testid="key-group-owner"]').text()).toBe('Production')
    expect(groups[0].findAll('[data-testid="key-row"]').map((row) => row.text())).toEqual([
      expect.stringContaining('High Usage Key'),
      expect.stringContaining('Production Key')
    ])
    expect(groups[1].get('[data-testid="key-group-owner"]').text()).toBe('Staging')
    expect(groups[0].findAll('[data-testid="key-activity"]')[0].classes()).toContain('bg-emerald-500')
    expect(wrapper.text()).toContain('Production Key')
    expect(wrapper.text()).toContain('yuan')
    expect(wrapper.text()).toContain('$5.00 / $25.00')
    expect(wrapper.text()).toContain('$40.00 / $100.00')
  })

  it('keeps reset times visible and expires the recent activity indicator', async () => {
    vi.useFakeTimers()
    const now = new Date('2026-07-15T08:00:00.000Z')
    vi.setSystemTime(now)
    listKeys.mockResolvedValue({
      items: [{
        ...apiKey,
        last_used_at: now.toISOString(),
        reset_5h_at: new Date(now.getTime() + 60 * 60 * 1000).toISOString(),
        reset_7d_at: new Date(now.getTime() + 24 * 60 * 60 * 1000).toISOString()
      }],
      total: 1,
      page: 1,
      page_size: 1000,
      pages: 1
    })

    const wrapper = mount(RateLimitOverviewPanel)
    await flushPromises()

    expect(wrapper.text().match(/admin\.dashboard\.rateLimits\.resetsAt/g)).toHaveLength(2)
    expect(wrapper.get('[data-testid="key-activity"]').classes()).toContain('bg-emerald-500')

    vi.advanceTimersByTime(5 * 60 * 1000 + 30_000)
    await wrapper.vm.$nextTick()

    expect(wrapper.get('[data-testid="key-activity"]').classes()).toContain('bg-gray-300')
    wrapper.unmount()
  })

  it('syncs the selected API keys to the selected upstream 7-day window', async () => {
    const wrapper = mount(RateLimitOverviewPanel)
    await flushPromises()

    const group = wrapper.findAll('[data-testid="key-group"]')[0]
    await group.get('[data-testid="sync-7d-window-button"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(1, 1000, expect.objectContaining({ group: '3', lite: '1' }))
    await group.get('[data-testid="sync-account-select"]').setValue('31')
    const checkboxes = group.findAll('[data-testid="batch-key-checkbox"]')
    await checkboxes[0].setValue(true)
    await group.get('[data-testid="confirm-batch-action"]').trigger('click')
    await flushPromises()

    expect(batchSync7dWindow).toHaveBeenCalledWith([23], 3, 31)
    expect(listKeys).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalled()
  })

  it('resets only the selected API key 7-day usage', async () => {
    const wrapper = mount(RateLimitOverviewPanel)
    await flushPromises()

    const group = wrapper.findAll('[data-testid="key-group"]')[0]
    await group.get('[data-testid="reset-7d-usage-button"]').trigger('click')
    const confirm = group.get('[data-testid="confirm-batch-action"]')
    expect(confirm.attributes('disabled')).toBeDefined()

    await group.findAll('[data-testid="batch-key-checkbox"]')[1].setValue(true)
    await confirm.trigger('click')
    await flushPromises()

    expect(batchReset7dUsage).toHaveBeenCalledWith([21], 3)
    expect(showSuccess).toHaveBeenCalled()
  })
})
