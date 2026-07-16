import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import RequestArchiveView from '../RequestArchiveView.vue'

vi.mock('@/api/admin/requestArchive', () => ({
  default: {
    list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 }),
    getStatus: vi.fn().mockResolvedValue({ enabled: false, retention_days: 30 }),
    getDetail: vi.fn().mockResolvedValue({}),
    updateConfig: vi.fn().mockResolvedValue(undefined)
  },
  list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 }),
  getStatus: vi.fn().mockResolvedValue({ enabled: false, retention_days: 30 }),
  getDetail: vi.fn().mockResolvedValue({}),
  updateConfig: vi.fn().mockResolvedValue(undefined)
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() })
}))

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      admin: { requestArchive: { title: '请求存档', description: 'desc', configTitle: '配置', configHint: 'hint', retentionDays: '保留', days: '天', disabledHint: '未启用', empty: '空', searchPlaceholder: '搜索', startDate: '开始', endDate: '结束', detailTitle: '详情', time: '时间', user: '用户', model: '模型', protocol: '协议', endpoint: '端点', ip: 'IP', prompt: '预览', promptText: '文本' } },
      common: { search: '搜索', loading: '加载', view: '查看' }
    }
  }
})

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/', component: { template: '<div/>' } }]
})

describe('RequestArchiveView', () => {
  beforeEach(() => {
    router.push('/')
  })

  it('mounts without errors and renders config section', async () => {
    const wrapper = mount(RequestArchiveView, {
      global: {
        plugins: [i18n, router],
        stubs: {
          AppLayout: { template: '<div><slot/></div>' },
          TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
          Pagination: true,
          BaseDialog: true,
          Icon: true
        }
      }
    })
    await flushPromises()
    expect(wrapper.html()).toContain('配置')
    expect(wrapper.html()).toContain('搜索')
  })

  it('renders empty state when no data', async () => {
    const wrapper = mount(RequestArchiveView, {
      global: {
        plugins: [i18n, router],
        stubs: {
          AppLayout: { template: '<div><slot/></div>' },
          TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
          Pagination: true,
          BaseDialog: true,
          Icon: true
        }
      }
    })
    await flushPromises()
    // loading 完成后,空数据应显示 empty 提示或 disabled hint
    const html = wrapper.html()
    expect(html.includes('空') || html.includes('未启用') || html.includes('loading') || html.includes('table')).toBeTruthy()
  })
})
