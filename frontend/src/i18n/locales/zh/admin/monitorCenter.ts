export default {
  monitorCenter: {
    title: '运维中心',
    description: '聚焦 OpenAI 上游、Sub2API 网关、真实链路、时延、并发与慢请求',
    timeRange: '全局时间范围',
    autoRefresh: '自动刷新 60 秒',
    lastUpdated: '最后更新 {time}',
    loadPartial: '{count} 组监控数据刷新失败。',
    keepPrevious: '已保留上一次成功数据。',
    ranges: { '1h': '1 小时', '6h': '6 小时', '24h': '24 小时', custom: '自定义' },
    custom: {
      start: '起始时间', end: '结束时间', apply: '应用', required: '请填写起始和结束时间。',
      invalid: '时间格式无效。', order: '起始时间必须早于结束时间。', tooLong: '单次查询最长支持 30 天。',
    },
    status: {
      operational: '正常', degraded_performance: '性能下降', partial_outage: '部分故障',
      major_outage: '严重故障', under_maintenance: '维护中', unknown: '未知',
    },
    common: { healthy: '正常', abnormal: '异常', unknown: '状态未知' },
    health: { good: '健康', warn: '关注', bad: '风险' },
    cockpit: {
      health: '健康', overallHealth: '综合健康', healthHint: '由网关 SLA、错误率、时延与系统资源共同计算',
      realtime: '实时吞吐', currentQps: '当前 QPS', currentTps: '当前 TPS', peakTps: '峰值 TPS', averageTps: '平均 TPS',
      requests: '请求数', tokensDetail: 'Token {value}', errorsDetail: 'SLA 异常 {value}', requestError: '请求错误率',
      businessLimitsDetail: '业务限制 {value}', e2eP99: '请求时长 P99', ttftP99: 'TTFT P99', upstreamError: '上游错误率',
      upstreamExclusions: '排除 429 / 529',
    },
    resources: {
      cpu: 'CPU', cpuHint: '警告阈值 80%', memory: '内存', database: '数据库', redis: 'Redis', goroutines: '协程', jobs: '后台任务',
      databaseDetail: '活跃 {active} · 空闲 {idle}', redisDetail: '总连接 {total} · 空闲 {idle}',
      goroutineDetail: '并发队列 {queue}', jobsDetail: '已监控 {count} 项',
    },
    upstream: {
      title: 'OpenAI 官方服务', incidentCount: '未解决事故 {count}', lastSync: '最后同步 {time}',
      stale: '官方状态本次同步失败，当前展示 {time} 的最后成功数据。', unavailable: '尚无可用的 OpenAI 官方状态数据。',
      notReported: '未报告', oneHourHistory: '最近一小时状态', rangeHistory: '最近{range}状态', coverage: '采样覆盖 {actual}/{expected} 分钟（{percent}%）',
      missingSample: '无采样', unresolvedIncident: '未解决事故', incidentMarker: '{count} 个未解决事故', noIncidents: '当前没有未解决事故',
      noIncidentsHint: 'OpenAI 当前未报告未解决事故',
      incidentMeta: '{status} · 影响 {impact} · 更新于 {time}', officialDetails: '官方详情', openOfficial: '打开 OpenAI Status',
    },
    gateway: {
      title: 'Sub2API 网关', subtitle: 'SLA、错误率与本地策略', requests: '请求数', errors: '错误数',
      errorRate: '请求错误', upstreamError: '上游错误', businessLimits: '业务限制',
      good: '稳定', warn: '波动', bad: '异常', unknown: '未知',
    },
    probe: {
      title: '真实链路探测', direct: '直连 OpenAI 官方端点', customEndpoint: '已指定的 Sub2API 链路监控', notConfigured: '请将一个非直连 OpenAI 渠道监控的分组设为 monitor-center',
      lastLatency: '最近探测耗时', failures: '连续失败 {count}', model: '探测模型', lastSuccess: '最近成功', latency: '探测耗时', noSamples: '所选时段暂无真实链路探测样本',
    },
    latency: {
      title: '服务请求响应时间', subtitle: 'P95、P90、P50、Avg 与 Max；空采样窗口不会被连接', mode: '时延指标', noData: '所选时段暂无时延样本',
    },
    concurrency: {
      title: '三通道用户并发', subtitle: '普通、重请求与恢复通道独立展示；用户选择只影响本模块',
      normal: '普通通道', heavy: '重请求通道', recovery: '恢复通道', selectUser: '选择用户曲线', noUsers: '无可选用户',
      partialCoverage: '并发快照仅保留 24 小时，当前图表显示后端可提供的有效覆盖范围。',
      current: '占用 {active} · 排队 {waiting}', demand: '并发需求（请求数）', responseTime: '响应时间',
      systemActive: '系统占用', systemQueue: '系统排队', tooltip: '需求 {demand}，占用 {active}，排队 {waiting}', laneEmpty: '该通道在所选时段暂无并发活动',
    },
    slow: {
      title: '慢请求根因诊断', primaryCause: '{cause} 占慢请求的 {share}', noSlowRequests: '所选时段暂无慢请求',
      ranking: '慢因排名', impact: '影响范围', impactHint: '加载全部用户、账号或模型，可滚动查看并按列排序', slowRate: '慢请求率',
      queueP95: '排队 P95', requests: '请求数', mainCause: '主要慢因', dimensions: { user: '用户', account: '账号', model: '模型' },
      sortBy: '按{column}排序',
      ingestionWarning: '性能遥测存在数据损失：丢弃 {dropped} 条，写入失败 {failed} 条。',
    },
    history: {
      title: '最近三天历史探测', subtitle: 'OpenAI 官方状态、网关请求健康与真实链路分别保存和展示', gateway: '网关', probe: '真实链路',
      samples: '采样次数', successRate: '获取成功率', averageLatency: '平均探测延迟', anomalies: '异常次数',
      officialSample: 'OpenAI 状态探测', threeDaysAgo: '3 天前', now: '现在', good: '总体稳定', warn: '存在波动', bad: '存在故障', unknown: '暂无数据',
    },
  },
}
