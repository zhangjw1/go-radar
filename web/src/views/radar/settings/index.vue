<template>
  <div class="container">
    <Breadcrumb :items="['menu.user', 'menu.radar.settings']" />
    <a-page-header
      title="运行设置"
      subtitle="扫描器开关、间隔、代理和推送配置"
    />
    <a-card :bordered="false">
      <a-alert
        v-if="errorMessage"
        class="error-alert"
        type="warning"
        :content="errorMessage"
        show-icon
      />
      <a-row class="toolbar" align="center" justify="space-between">
        <a-col>
          <a-space>
            <a-button type="primary" @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              刷新
            </a-button>
            <a-tag color="arcoblue">{{ activeRows.length }} 项</a-tag>
          </a-space>
        </a-col>
      </a-row>
      <a-tabs v-model:active-key="activeCategory" type="rounded">
        <a-tab-pane
          v-for="item in categories"
          :key="item.key"
          :title="item.title"
        >
          <a-table
            row-key="key"
            :columns="columns"
            :data="activeRows"
            :loading="loading"
            :pagination="{ pageSize: 12 }"
            :bordered="false"
          >
            <template #name="{ record }">
              <a-space direction="vertical" :size="0">
                <span>{{ record.name }}</span>
                <span class="setting-desc">{{ record.description }}</span>
              </a-space>
            </template>
            <template #key="{ record }">
              <a-tag>{{ record.key }}</a-tag>
            </template>
            <template #value="{ record }">
              <a-tag :color="valueColor(record.value)">
                {{ formatValue(record.value) }}
              </a-tag>
            </template>
            <template #defaultValue="{ record }">
              {{ formatValue(record.defaultValue) }}
            </template>
            <template #override="{ record }">
              <span v-if="record.override !== ''">
                {{ formatValue(record.override) }}
              </span>
              <span v-else class="empty-value">未覆盖</span>
            </template>
          </a-table>
        </a-tab-pane>
      </a-tabs>
    </a-card>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue';
  import { Message } from '@arco-design/web-vue';
  import type { TableColumnData } from '@arco-design/web-vue/es/table/interface';
  import { querySettings } from '@/api/radar';

  type CategoryKey =
    | 'scanner'
    | 'push'
    | 'proxy'
    | 'strategy'
    | 'market'
    | 'other';

  type SettingRow = {
    key: string;
    name: string;
    description: string;
    category: CategoryKey;
    value: string;
    defaultValue: string;
    override: string;
  };

  const categories: Array<{ key: CategoryKey; title: string }> = [
    { key: 'scanner', title: '扫描器' },
    { key: 'strategy', title: '策略参数' },
    { key: 'push', title: '推送' },
    { key: 'proxy', title: '网络代理' },
    { key: 'market', title: '行情接口' },
    { key: 'other', title: '其他' },
  ];

  const settingMeta: Record<
    string,
    { name: string; description: string; category: CategoryKey }
  > = {
    enable_binance_square: {
      name: '启用币安广场',
      description: '是否抓取币安广场内容作为辅助信号源',
      category: 'market',
    },
    enable_scanner_s1: {
      name: '启用 S1 币安公告',
      description: '监听币安公告带来的事件催化信号',
      category: 'scanner',
    },
    enable_scanner_s2: {
      name: '启用 S2 费率翻转',
      description: '监控资金费率翻转和持仓变化',
      category: 'scanner',
    },
    enable_scanner_s3: {
      name: '启用 S3 热度确认',
      description: '监控成交额、持仓和热度确认信号',
      category: 'scanner',
    },
    enable_scanner_s5: {
      name: '启用 S5 链上发现',
      description: '监控链上新币和动量变化',
      category: 'scanner',
    },
    enable_scanner_s7: {
      name: '启用 S7 Vitalik Sell',
      description: '监控指定 ERC-20 转出路径',
      category: 'scanner',
    },
    scan_interval_s1: {
      name: 'S1 扫描间隔',
      description: 'S1 扫描器运行间隔，单位秒',
      category: 'scanner',
    },
    scan_interval_s2: {
      name: 'S2 扫描间隔',
      description: 'S2 扫描器运行间隔，单位秒',
      category: 'scanner',
    },
    scan_interval_s3: {
      name: 'S3 扫描间隔',
      description: 'S3 扫描器运行间隔，单位秒',
      category: 'scanner',
    },
    scan_interval_s5: {
      name: 'S5 扫描间隔',
      description: 'S5 扫描器运行间隔，单位秒',
      category: 'scanner',
    },
    scan_interval_s7: {
      name: 'S7 扫描间隔',
      description: 'S7 扫描器运行间隔，单位秒',
      category: 'scanner',
    },
    tg_proxy_url: {
      name: 'Telegram 代理',
      description: 'Telegram Bot 请求使用的代理地址',
      category: 'push',
    },
    token_push_cooldown_minutes: {
      name: 'Token 推送冷却',
      description: '同一 Token 再次推送的最小间隔，单位分钟',
      category: 'push',
    },
    watchlist_cooldown_minutes: {
      name: '观察名单冷却',
      description: '观察名单 Token 推送冷却时间，单位分钟',
      category: 'push',
    },
    s3_digest_cooldown_minutes: {
      name: 'S3 摘要冷却',
      description: 'S3 摘要推送的冷却时间，单位分钟',
      category: 'push',
    },
    selective_proxy_url: {
      name: '选择性代理地址',
      description: '仅对指定域名启用的代理地址',
      category: 'proxy',
    },
    selective_proxy_domains: {
      name: '选择性代理域名',
      description: '使用选择性代理的域名列表',
      category: 'proxy',
    },
    http_trust_env: {
      name: '读取系统代理',
      description: '是否信任系统 HTTP_PROXY 等环境变量',
      category: 'proxy',
    },
    gmgn_timeout_seconds: {
      name: 'GMGN 超时',
      description: 'GMGN 接口请求超时时间，单位秒',
      category: 'market',
    },
    gmgn_retries: {
      name: 'GMGN 重试次数',
      description: 'GMGN 接口失败后的重试次数',
      category: 'market',
    },
    s7_eth_rpc_url: {
      name: 'S7 ETH RPC',
      description: 'S7 扫描器使用的以太坊 RPC 地址',
      category: 'market',
    },
    signal_time_bucket_minutes: {
      name: '信号去重时间桶',
      description: '同类信号去重窗口，单位分钟',
      category: 'strategy',
    },
    resonance_lookback_minutes: {
      name: '共振回看窗口',
      description: '跨源共振检查的回看窗口，单位分钟',
      category: 'strategy',
    },
    s3_min_oi_delta_pct: {
      name: 'S3 最小持仓增幅',
      description: 'S3 信号触发所需持仓增幅百分比',
      category: 'strategy',
    },
    s3_min_oi_usd: {
      name: 'S3 最小持仓规模',
      description: 'S3 信号触发所需持仓规模，单位美元',
      category: 'strategy',
    },
    s3_min_vol_usd: {
      name: 'S3 最小成交额',
      description: 'S3 信号触发所需成交额，单位美元',
      category: 'strategy',
    },
    s3_vol_surge_mult: {
      name: 'S3 放量倍数',
      description: 'S3 判断成交额放大的倍数阈值',
      category: 'strategy',
    },
    s5_min_gain_pct: {
      name: 'S5 最小涨幅',
      description: 'S5 动量信号触发所需涨幅百分比',
      category: 'strategy',
    },
    s5_momentum_consecutive_up: {
      name: 'S5 连续上涨次数',
      description: 'S5 动量确认所需连续上涨次数',
      category: 'strategy',
    },
    s5_momentum_medium_quota: {
      name: 'S5 中优先级额度',
      description: 'S5 中优先级动量信号额度',
      category: 'strategy',
    },
    s7_min_notify_usd: {
      name: 'S7 最小通知金额',
      description: 'S7 触发通知的最小美元金额',
      category: 'strategy',
    },
  };

  const loading = ref(false);
  const settings = ref<SettingRow[]>([]);
  const errorMessage = ref('');
  const activeCategory = ref<CategoryKey>('scanner');

  const columns: TableColumnData[] = [
    { title: '配置项', dataIndex: 'name', slotName: 'name', width: 260 },
    { title: '字段 key', dataIndex: 'key', slotName: 'key', width: 230 },
    { title: '当前值', dataIndex: 'value', slotName: 'value', width: 160 },
    {
      title: '默认值',
      dataIndex: 'defaultValue',
      slotName: 'defaultValue',
      width: 160,
    },
    {
      title: '覆盖值',
      dataIndex: 'override',
      slotName: 'override',
      width: 160,
    },
  ];

  const activeRows = computed(() =>
    settings.value.filter((item) => item.category === activeCategory.value)
  );

  const fallbackName = (key: string) =>
    key
      .split('_')
      .map((part) => part.toUpperCase())
      .join(' ');

  const inferCategory = (key: string): CategoryKey => {
    if (key.startsWith('enable_scanner') || key.startsWith('scan_interval')) {
      return 'scanner';
    }
    if (key.includes('proxy') || key === 'http_trust_env') return 'proxy';
    if (key.includes('cooldown') || key.startsWith('tg_')) return 'push';
    if (key.startsWith('gmgn') || key.includes('rpc')) return 'market';
    if (key.startsWith('s') || key.includes('resonance')) return 'strategy';
    return 'other';
  };

  const normalizeRow = (
    key: string,
    value: string | number | boolean,
    defaultValue: string | number | boolean | undefined,
    overrides: Record<string, string>
  ): SettingRow => {
    const meta = settingMeta[key] || {
      name: fallbackName(key),
      description: '待补充配置说明',
      category: inferCategory(key),
    };
    return {
      key,
      name: meta.name,
      description: meta.description,
      category: meta.category,
      value: String(value),
      defaultValue: String(defaultValue ?? value),
      override:
        Object.prototype.hasOwnProperty.call(overrides, key) &&
        overrides[key] !== undefined
          ? String(overrides[key])
          : '',
    };
  };

  const formatValue = (value?: string) => {
    if (value === undefined || value === '') return '-';
    if (value === 'true') return '开启';
    if (value === 'false') return '关闭';
    return value;
  };

  const valueColor = (value?: string) => {
    if (value === 'true') return 'green';
    if (value === 'false') return 'red';
    return 'arcoblue';
  };

  async function fetchData() {
    loading.value = true;
    errorMessage.value = '';
    try {
      const res = await querySettings();
      const payload = res.data?.settings || [];
      const overrides = res.data?.overrides || {};
      settings.value = Array.isArray(payload)
        ? payload.map((item) =>
            normalizeRow(
              item.key,
              item.value,
              item.default,
              overrides as Record<string, string>
            )
          )
        : Object.entries(payload).map(([key, value]) =>
            normalizeRow(key, value, value, overrides)
          );
    } catch (err) {
      settings.value = [];
      errorMessage.value =
        '运行设置加载失败，请确认 Go 后端已启动并使用最新版本。';
      Message.error('运行设置加载失败');
    } finally {
      loading.value = false;
    }
  }

  onMounted(fetchData);
</script>

<style scoped lang="less">
  .container {
    padding: 0 20px 20px 20px;
  }

  :deep(.arco-page-header) {
    padding-right: 0;
    padding-left: 0;
  }

  .toolbar {
    margin-bottom: 16px;
  }

  .error-alert {
    margin-bottom: 16px;
  }

  .setting-desc {
    color: var(--color-text-3);
    font-size: 12px;
    line-height: 20px;
  }

  .empty-value {
    color: var(--color-text-4);
  }
</style>
