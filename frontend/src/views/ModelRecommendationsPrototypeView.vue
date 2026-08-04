<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-gray-100">
    <header class="border-b border-gray-200 bg-white/90 px-4 py-4 backdrop-blur-sm dark:border-dark-800 dark:bg-dark-900/90 sm:px-6">
      <div class="mx-auto flex max-w-[1600px] items-center justify-between gap-4">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary-100 dark:bg-primary-900/30">
            <Icon name="sparkles" size="sm" class="text-primary-600 dark:text-primary-400" />
          </div>
          <div class="min-w-0">
            <h1 class="truncate text-base font-semibold text-gray-900 dark:text-white">模型推荐 · 原型预览</h1>
            <p class="truncate text-xs text-gray-500 dark:text-gray-400">仪表盘推荐模块的新版信息架构</p>
          </div>
        </div>
        <router-link to="/home" class="btn btn-secondary btn-sm shrink-0">
          返回首页
        </router-link>
      </div>
    </header>

    <main class="mx-auto max-w-[1600px] px-4 py-5 sm:px-6 lg:py-8">
      <UserDashboardModelRecommendations :data="recommendations" @refresh="refreshPreview" />
      <p class="mt-3 text-center text-[11px] text-gray-400 dark:text-gray-500">
        预览数据仅用于展示布局，接入仪表盘后由推荐接口提供。
      </p>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import UserDashboardModelRecommendations from '@/components/user/dashboard/UserDashboardModelRecommendations.vue'
import type { CodexRadarDashboardRecommendations } from '@/api/usage'

const recommendations = ref<CodexRadarDashboardRecommendations>({
  source_updated_at: '2026-08-03T23:52:00+08:00',
  station_available: true,
  intelligence_available: true,
  station_recommendations: [
    {
      key: 'daily_development',
      title: 'Daily development',
      items: [
        { model: 'gpt-5.6-sol', effort: 'xhigh', iq: 101.8, average_cost_usd: 6.16, average_duration_minutes: 26 },
        { model: 'deepseek-v4-flash', effort: 'max', iq: 92.4, average_cost_usd: 0.1, average_duration_minutes: 27 }
      ]
    },
    {
      key: 'hard_problems',
      title: 'Hard problems',
      items: [
        { model: 'gpt-5.6-sol', effort: 'max', iq: 107.1, average_cost_usd: 9.57, average_duration_minutes: 35 },
        { model: 'gpt-5.6-sol', effort: 'ultra', iq: 105.8, average_cost_usd: 21.98, average_duration_minutes: 53 }
      ]
    },
    {
      key: 'background_automation',
      title: 'Background automation',
      items: [
        { model: 'deepseek-v4-flash', effort: 'max', iq: 92.4, average_cost_usd: 0.1, average_duration_minutes: 27 },
        { model: 'gpt-5.6-luna', effort: 'xhigh', iq: 92.4, average_cost_usd: 0.31, average_duration_minutes: 22 }
      ]
    },
    {
      key: 'lobster_tasks',
      title: 'Lobster tasks',
      items: [
        { model: 'gpt-5.6-terra', effort: 'medium', iq: 61.6, average_cost_usd: 0.63, average_duration_minutes: 9.5 },
        { model: 'gpt-5.6-luna', effort: 'high', iq: 64.3, average_cost_usd: 0.2, average_duration_minutes: 16 }
      ]
    }
  ],
  intelligence_recommendations: [
    { model: 'gpt-5.6-sol', effort: 'ultra', iq: 105.8, samples: 112, average_cost_usd: 21.89, average_duration_minutes: 53 },
    { model: 'gpt-5.6-sol', effort: 'max', iq: 107.1, samples: 112, average_cost_usd: 9.57, average_duration_minutes: 35 },
    { model: 'gpt-5.6-sol', effort: 'xhigh', iq: 101.8, samples: 112, average_cost_usd: 6.12, average_duration_minutes: 26 },
    { model: 'gpt-5.6-sol', effort: 'high', iq: 81.7, samples: 112, average_cost_usd: 4.98, average_duration_minutes: 23 },
    { model: 'gpt-5.6-sol', effort: 'medium', iq: 89.7, samples: 112, average_cost_usd: 3.62, average_duration_minutes: 17 },
    { model: 'gpt-5.6-sol', effort: 'low', iq: 76.3, samples: 112, average_cost_usd: 2.01, average_duration_minutes: 11 },
    { model: 'gpt-5.6-terra', effort: 'ultra', iq: 97.8, samples: 112, average_cost_usd: 9.69, average_duration_minutes: 43 },
    { model: 'gpt-5.6-terra', effort: 'max', iq: 96.4, samples: 112, average_cost_usd: 4.02, average_duration_minutes: 32 },
    { model: 'gpt-5.6-terra', effort: 'xhigh', iq: 85.7, samples: 112, average_cost_usd: 1.86, average_duration_minutes: 18 },
    { model: 'gpt-5.6-terra', effort: 'high', iq: 69.6, samples: 112, average_cost_usd: 1.18, average_duration_minutes: 14 },
    { model: 'gpt-5.6-terra', effort: 'medium', iq: 61.6, samples: 112, average_cost_usd: 0.63, average_duration_minutes: 9.5 },
    { model: 'gpt-5.6-terra', effort: 'low', iq: 44.2, samples: 112, average_cost_usd: 0.48, average_duration_minutes: 8.7 },
    { model: 'gpt-5.6-luna', effort: 'max', iq: 101.8, samples: 112, average_cost_usd: 0.47, average_duration_minutes: 33 },
    { model: 'gpt-5.6-luna', effort: 'xhigh', iq: 92.4, samples: 112, average_cost_usd: 0.31, average_duration_minutes: 22 },
    { model: 'gpt-5.6-luna', effort: 'high', iq: 64.3, samples: 112, average_cost_usd: 0.2, average_duration_minutes: 16 },
    { model: 'gpt-5.6-luna', effort: 'medium', iq: 34.8, samples: 112, average_cost_usd: 0.09, average_duration_minutes: 9.1 },
    { model: 'gpt-5.6-luna', effort: 'low', iq: 8, samples: 112, average_cost_usd: 0.03, average_duration_minutes: 4.9 },
    { model: 'gpt-5.5', effort: 'xhigh', iq: 99.1, samples: 112, average_cost_usd: 5.73, average_duration_minutes: 23 },
    { model: 'gpt-5.5', effort: 'high', iq: 84.4, samples: 112, average_cost_usd: 3.6, average_duration_minutes: 17 },
    { model: 'deepseek-v4-flash', effort: 'max', iq: 92.4, samples: 112, average_cost_usd: 0.1, average_duration_minutes: 27 },
    { model: 'deepseek-v4-flash', effort: 'high', iq: 73.7, samples: 112, average_cost_usd: 0.1, average_duration_minutes: 27 }
  ]
})

function refreshPreview() {
  recommendations.value = {
    ...recommendations.value,
    source_updated_at: new Date().toISOString()
  }
}
</script>
