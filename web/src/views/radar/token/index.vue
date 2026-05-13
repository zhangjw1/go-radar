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
              <a-descriptions-item label="地址">
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
                <a-statistic title="相关信号" :value="signals.length" />
              </a-grid-item>
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <a-statistic title="快照数量" :value="snapshots.length" />
              </a-grid-item>
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <a-statistic
                  title="最高分数"
                  :value="highestScore ? formatScore(highestScore) : '-'"
                />
              </a-grid-item>
              <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
                <a-statistic
                  title="观察状态"
                  :value="watchItem?.status || '未加入'"
                />
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
              :data="signals"
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
                    {{ signalTypeLabel(record.signal_type) }}
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
            <a-table
              :data="snapshots"
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
                <a-table-column title="价格" data-index="price" />
                <a-table-column title="市值" data-index="mc" />
                <a-table-column title="流动性" data-index="liq" />
                <a-table-column title="成交量" data-index="volume" />
                <a-table-column title="资金费率" data-index="funding_pct" />
                <a-table-column title="OI" data-index="oi_usd" />
              </template>
            </a-table>
          </a-tab-pane>
        </a-tabs>
      </a-card>
    </a-space>
  </div>
</template>

<script lang="ts" setup>
  import { computed, onMounted, ref } from 'vue';
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
    signalTypeLabel,
    summarizeReason,
  } from '../shared';

  const route = useRoute();
  const router = useRouter();
  const loading = ref(false);
  const token = ref<TokenProfile>();
  const signals = ref<SignalEvent[]>([]);
  const snapshots = ref<TokenSnapshot[]>([]);
  const watchItem = ref<WatchlistItem | null>(null);

  const tokenTitle = computed(
    () => token.value?.name || token.value?.symbol || '标的详情'
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
    if (!signals.value.length) return 0;
    return Math.max(...signals.value.map((item) => item.score || 0));
  });

  const loadData = async () => {
    loading.value = true;
    try {
      const chain = String(route.params.chain || '');
      const address = String(route.params.address || '');
      const { data } = await queryToken(chain, address);
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
</style>
