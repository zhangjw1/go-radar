<template>
  <div class="container">
    <Breadcrumb :items="['menu.list', 'menu.radar.signals']" />
    <a-space direction="vertical" fill size="large">
      <a-page-header
        :title="tokenTitle"
        :subtitle="token?.address || '查看标的详情、快照与相关信号。'"
      >
        <template #extra>
          <a-space>
            <a-tag v-if="token?.chain" color="arcoblue">{{
              chainLabel(token.chain)
            }}</a-tag>
            <a-button size="small" @click="router.back()">返回</a-button>
          </a-space>
        </template>
      </a-page-header>

      <a-grid :cols="24" :col-gap="16" :row-gap="16">
        <a-grid-item :span="{ xs: 24, lg: 8 }">
          <a-card class="general-card info-card" title="基础信息">
            <a-descriptions :column="1" bordered size="large">
              <a-descriptions-item label="Symbol">
                {{ token?.symbol || '-' }}
              </a-descriptions-item>
              <a-descriptions-item label="名称">
                {{ token?.name || '-' }}
              </a-descriptions-item>
              <a-descriptions-item label="链/市场">
                {{ token?.chain ? chainLabel(token.chain) : '-' }}
              </a-descriptions-item>
              <a-descriptions-item label="叙事主题">
                {{ token?.narrative_theme || '-' }}
              </a-descriptions-item>
              <a-descriptions-item label="标签">
                <a-space wrap>
                  <a-tag v-for="tag in tags" :key="tag" color="arcoblue">
                    {{ tag }}
                  </a-tag>
                  <span v-if="!tags.length">-</span>
                </a-space>
              </a-descriptions-item>
              <a-descriptions-item :label="identifierLabel">
                <a-typography-paragraph copyable>
                  {{ token?.address || '-' }}
                </a-typography-paragraph>
              </a-descriptions-item>
              <a-descriptions-item label="描述">
                {{ token?.description || '-' }}
              </a-descriptions-item>
            </a-descriptions>
          </a-card>
        </a-grid-item>

        <a-grid-item :span="{ xs: 24, lg: 16 }">
          <a-card
            class="general-card"
            :title="watchItem ? '观察名单' : '状态概览'"
          >
            <a-grid :cols="24" :col-gap="16" :row-gap="16">
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <a-statistic title="相关信号" :value="visibleSignals.length" />
              </a-grid-item>
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <a-statistic
                  title="快照数量"
                  :value="visibleSnapshots.length"
                />
              </a-grid-item>
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <a-statistic title="最高分数" :value="highestScore" />
              </a-grid-item>
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <div class="status-summary">
                  <div class="status-summary__label">观察状态</div>
                  <div class="status-summary__value">
                    {{ watchItem?.status || '未加入' }}
                  </div>
                </div>
              </a-grid-item>
            </a-grid>

            <div v-if="watchItem" class="watch-note">
              <a-typography-text type="secondary">
                {{ watchItem.note || '暂无备注' }}
              </a-typography-text>
            </div>

            <div v-if="socialLinks.length" class="social-links">
              <a-space wrap>
                <a-tag
                  v-for="link in socialLinks"
                  :key="link.key"
                  color="orangered"
                >
                  {{ link.key }}: {{ link.value }}
                </a-tag>
              </a-space>
            </div>
          </a-card>
        </a-grid-item>
      </a-grid>

      <a-card class="general-card">
        <a-tabs default-active-key="signals" type="rounded">
          <a-tab-pane key="signals" title="相关信号">
            <a-table
              :data="visibleSignals"
              :loading="loading"
              row-key="id"
              :pagination="{ pageSize: 10 }"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="时间" data-index="created_at">
                  <template #cell="{ record }">
                    {{ formatTime(record.created_at) }}
                  </template>
                </a-table-column>
                <a-table-column title="来源" data-index="source" />
                <a-table-column title="信号类型">
                  <template #cell="{ record }">
                    {{ signalTypeLabelForRecord(record) }}
                  </template>
                </a-table-column>
                <a-table-column title="优先级" data-index="priority">
                  <template #cell="{ record }">
                    <a-tag :color="priorityColor(record.priority)">
                      {{ priorityLabel(record.priority) }}
                    </a-tag>
                  </template>
                </a-table-column>
                <a-table-column title="分数" data-index="score">
                  <template #cell="{ record }">
                    {{ formatScore(record.score) }}
                  </template>
                </a-table-column>
                <a-table-column title="说明">
                  <template #cell="{ record }">
                    {{ summarizeReason(record.signal_type, record.reason) }}
                  </template>
                </a-table-column>
              </template>
            </a-table>
          </a-tab-pane>

          <a-tab-pane key="snapshots" title="快照">
            <a-tabs
              v-if="snapshotSources.length > 1"
              v-model:active-key="activeSnapshotSource"
              type="rounded"
              size="small"
              class="source-tabs"
            >
              <a-tab-pane
                v-for="source in snapshotSources"
                :key="source"
                :title="sourceTabTitle(source)"
              />
            </a-tabs>
            <a-table
              :data="visibleSnapshots"
              :loading="loading"
              row-key="id"
              :pagination="{ pageSize: 10 }"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="时间" data-index="created_at">
                  <template #cell="{ record }">
                    {{ formatTime(record.created_at) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotSourceColumn"
                  title="来源"
                  data-index="source"
                />
                <a-table-column
                  v-if="showSnapshotPriceColumn"
                  title="价格"
                  data-index="price"
                >
                  <template #cell="{ record }">
                    {{ formatPrice(record.price) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotMarketCapColumn"
                  title="市值"
                  data-index="mc"
                >
                  <template #cell="{ record }">
                    {{ formatCompactUnit(record.mc) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotHoldersColumn"
                  title="持仓人数"
                  data-index="holders"
                >
                  <template #cell="{ record }">
                    {{ formatOptionalNumber(record.holders) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotLiquidityColumn"
                  title="流动性"
                  data-index="liq"
                >
                  <template #cell="{ record }">
                    {{ formatCompactUnit(record.liq) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotVolumeColumn"
                  title="成交量"
                  data-index="volume"
                >
                  <template #cell="{ record }">
                    {{ formatCompactUnit(record.volume) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotFundingColumn"
                  title="资金费率"
                  data-index="funding_pct"
                >
                  <template #cell="{ record }">
                    <a-tooltip :content="fundingTooltip(record)">
                      <span>{{ formatFunding(record) }}</span>
                    </a-tooltip>
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotOIColumn"
                  title="OI"
                  data-index="oi_usd"
                >
                  <template #cell="{ record }">
                    {{ formatCompactUnit(record.oi_usd) }}
                  </template>
                </a-table-column>
                <a-table-column
                  v-if="showSnapshotOIDeltaColumn"
                  title="6小时 OI 对比"
                  data-index="oi_d6h"
                >
                  <template #cell="{ record }">
                    <a-tooltip :content="oiCompareTooltip(record)">
                      <span>{{ formatOIDelta(record) }}</span>
                    </a-tooltip>
                  </template>
                </a-table-column>
              </template>
            </a-table>
          </a-tab-pane>
        </a-tabs>
      </a-card>
    </a-space>
  </div>
</template>

<script lang="ts" setup>
  import { computed, onMounted, ref, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { Message } from '@arco-design/web-vue';
  import {
    queryToken,
    type SignalEvent,
    type TokenProfile,
    type TokenSnapshot,
    type WatchlistItem,
  } from '@/api/radar';
  import {
    chainLabel,
    formatScore,
    formatTime,
    priorityColor,
    priorityLabel,
    signalTypeLabelForRecord,
    summarizeReason,
  } from '../shared';

  const route = useRoute();
  const router = useRouter();
  const loading = ref(false);
  const token = ref<TokenProfile>();
  const signals = ref<SignalEvent[]>([]);
  const snapshots = ref<TokenSnapshot[]>([]);
  const watchItem = ref<WatchlistItem | null>(null);
  const activeSnapshotSource = ref('all');

  const routeSource = computed(() => String(route.query.source || ''));

  const snapshotSources = computed(() => {
    const sources = Array.from(
      new Set(snapshots.value.map((item) => item.source).filter(Boolean))
    );
    return sources.length > 1 ? ['all', ...sources] : sources;
  });

  const sourceTabTitle = (source: string) =>
    source === 'all' ? '全部' : source.toUpperCase();

  const visibleSnapshots = computed(() => {
    if (activeSnapshotSource.value && activeSnapshotSource.value !== 'all') {
      return snapshots.value.filter(
        (item) => item.source === activeSnapshotSource.value
      );
    }
    return snapshots.value;
  });

  const visibleSignals = computed(() => {
    if (routeSource.value) {
      return signals.value.filter((item) => item.source === routeSource.value);
    }
    return signals.value;
  });

  const hasSnapshotValue = (getter: (item: TokenSnapshot) => unknown) =>
    visibleSnapshots.value.some((item) => {
      const value = getter(item);
      return value !== null && value !== undefined && value !== '';
    });

  const showSnapshotSourceColumn = computed(() => {
    const sources = new Set(
      visibleSnapshots.value.map((item) => item.source).filter(Boolean)
    );
    return sources.size > 1;
  });
  const showSnapshotPriceColumn = computed(() =>
    hasSnapshotValue((item) => item.price)
  );
  const showSnapshotMarketCapColumn = computed(() =>
    hasSnapshotValue((item) => item.mc)
  );
  const showSnapshotHoldersColumn = computed(() =>
    hasSnapshotValue((item) => item.holders)
  );
  const showSnapshotLiquidityColumn = computed(() =>
    hasSnapshotValue((item) => item.liq)
  );
  const showSnapshotVolumeColumn = computed(() =>
    hasSnapshotValue((item) => item.volume)
  );
  const showSnapshotOIColumn = computed(() =>
    hasSnapshotValue((item) => item.oi_usd)
  );
  const showSnapshotOIDeltaColumn = computed(() =>
    hasSnapshotValue((item) => item.oi_d6h)
  );

  const tokenTitle = computed(
    () => token.value?.name || token.value?.symbol || '标的详情'
  );
  const identifierLabel = computed(() =>
    token.value?.chain === 'binance_perp' ? '合约交易对' : '地址'
  );

  const tags = computed(() => {
    try {
      return token.value?.narrative_tags_json
        ? JSON.parse(token.value.narrative_tags_json)
        : [];
    } catch {
      return [];
    }
  });

  const socialLinks = computed(() => {
    try {
      const raw = token.value?.social_links_json
        ? JSON.parse(token.value.social_links_json)
        : {};
      return Object.entries(raw).map(([key, value]) => ({
        key,
        value: String(value),
      }));
    } catch {
      return [];
    }
  });

  const highestScore = computed(() => {
    if (!visibleSignals.value.length) return 0;
    return Math.max(...visibleSignals.value.map((item) => item.score || 0));
  });

  const toFiniteNumber = (value?: number | null) => {
    if (value === null || value === undefined) return '-';
    const numeric = Number(value);
    if (Number.isNaN(numeric)) return '-';
    return numeric;
  };

  const formatPrice = (value?: number | null) => {
    const numeric = toFiniteNumber(value);
    if (numeric === '-') return '-';
    return numeric.toLocaleString('zh-CN', {
      maximumFractionDigits: numeric >= 1 ? 6 : 10,
    });
  };

  const formatCompactUnit = (value?: number | null) => {
    const numeric = toFiniteNumber(value);
    if (numeric === '-') return '-';
    const abs = Math.abs(numeric);
    if (abs >= 100000000) {
      return `${(numeric / 100000000).toLocaleString('zh-CN', {
        maximumFractionDigits: 2,
      })} 亿`;
    }
    if (abs >= 10000) {
      return `${(numeric / 10000).toLocaleString('zh-CN', {
        maximumFractionDigits: 2,
      })} 万`;
    }
    return numeric.toLocaleString('zh-CN', { maximumFractionDigits: 2 });
  };

  const formatOptionalNumber = (value?: number | null) => {
    const numeric = toFiniteNumber(value);
    if (numeric === '-') return '-';
    return numeric.toLocaleString('zh-CN');
  };

  const formatPercent = (value?: number | null) => {
    const numeric = toFiniteNumber(value);
    if (numeric === '-') return '-';
    return `${numeric.toLocaleString('zh-CN', {
      maximumFractionDigits: 4,
    })}%`;
  };

  const parseSnapshotRaw = (rawJson?: string) => {
    if (!rawJson) return {};
    try {
      return JSON.parse(rawJson) as Record<string, unknown>;
    } catch {
      return {};
    }
  };

  const hasFundingValue = (record: TokenSnapshot) => {
    const raw = parseSnapshotRaw(record.raw_json);
    if (raw.has_funding === true) return true;
    if (raw.has_funding === false) return false;
    if (record.funding_pct === null || record.funding_pct === undefined) {
      return false;
    }
    return 'unknown';
  };

  const showSnapshotFundingColumn = computed(() =>
    visibleSnapshots.value.some((item) => hasFundingValue(item))
  );

  const formatFunding = (record: TokenSnapshot) => {
    const hasFunding = hasFundingValue(record);
    if (!hasFunding) return '-';
    return formatPercent(record.funding_pct);
  };

  const fundingTooltip = (record: TokenSnapshot) => {
    const hasFunding = hasFundingValue(record);
    if (hasFunding === true) return '已获取 Binance 资金费率';
    if (hasFunding === false) return '本轮未获取到资金费率';
    return '历史数据未记录是否获取到资金费率';
  };

  const formatOIDelta = (record: TokenSnapshot) => {
    const numeric = toFiniteNumber(record.oi_d6h);
    if (numeric === '-') return '-';
    const sign = numeric > 0 ? '+' : '';
    return `${sign}${numeric.toLocaleString('zh-CN', {
      maximumFractionDigits: 1,
    })}%`;
  };

  const oiCompareTooltip = (record: TokenSnapshot) => {
    const raw = parseSnapshotRaw(record.raw_json);
    const history = Array.isArray(raw.oi_history_usd)
      ? raw.oi_history_usd
          .map((item) => Number(item))
          .filter((item) => !Number.isNaN(item))
      : [];
    const previous = toFiniteNumber(raw.oi_prev6h_usd as number | null);
    const current = toFiniteNumber(record.oi_usd);
    const points = toFiniteNumber(raw.oi_history_points as number | null);
    const delta = formatOIDelta(record);
    if (history.length >= 2 && delta !== '-') {
      return `Binance OI序列：${history
        .map((item) => formatCompactUnit(item))
        .join(' -> ')}，变化 ${delta}`;
    }
    if (previous === '-' || current === '-' || delta === '-') {
      return '当前 OI；旧快照可能没有记录 Binance 小时级窗口基准值';
    }
    const pointText = points === '-' ? '' : `，样本点 ${points}`;
    return `当前 ${formatCompactUnit(current)}，窗口基准 ${formatCompactUnit(
      previous
    )}，变化 ${delta}${pointText}`;
  };

  const loadData = async () => {
    loading.value = true;
    try {
      const chain = String(route.params.chain || '');
      const address = String(route.params.address || '');
      const source = routeSource.value;
      const { data } = await queryToken(
        chain,
        address,
        source ? { source } : undefined
      );
      token.value = data.token;
      signals.value = data.signals || [];
      snapshots.value = data.snapshots || [];
      watchItem.value = data.watch_item;
    } catch (err) {
      Message.error('标的详情加载失败');
    } finally {
      loading.value = false;
    }
  };

  watch(
    [routeSource, snapshotSources],
    ([source, sources]) => {
      if (source && sources.includes(source)) {
        activeSnapshotSource.value = source;
        return;
      }
      if (!sources.includes(activeSnapshotSource.value)) {
        activeSnapshotSource.value = sources[0] || 'all';
      }
    },
    { immediate: true }
  );

  onMounted(loadData);
</script>

<style scoped lang="less">
  .container {
    padding: 0 20px 20px 20px;
  }

  :deep(.arco-page-header) {
    padding-right: 0;
    padding-left: 0;
  }

  .info-card :deep(.arco-descriptions-item-label) {
    width: 96px;
  }

  .watch-note {
    margin-top: 16px;
  }

  .social-links {
    margin-top: 16px;
  }

  .source-tabs {
    margin-bottom: 12px;
  }

  .status-summary {
    padding-top: 2px;
  }

  .status-summary__label {
    color: var(--color-text-2);
    font-size: 14px;
    line-height: 22px;
  }

  .status-summary__value {
    margin-top: 6px;
    color: var(--color-text-1);
    font-size: 30px;
    line-height: 38px;
  }
</style>
