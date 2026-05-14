<template>
  <div class="radar-page">
    <a-page-header title="Web3 Radar" subtitle="链上雷达监控总览" />

    <a-grid :cols="24" :col-gap="16" :row-gap="16">
      <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
        <a-card>
          <a-statistic title="最近信号" :value="signals.length" />
        </a-card>
      </a-grid-item>
      <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
        <a-card>
          <a-statistic title="高优先级" :value="highSignals.length" />
        </a-card>
      </a-grid-item>
      <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
        <a-card>
          <a-statistic title="任务记录" :value="jobs.length" />
        </a-card>
      </a-grid-item>
      <a-grid-item :span="{ xs: 24, sm: 12, lg: 6 }">
        <a-card>
          <a-statistic title="观察名单" :value="watchlist.length" />
        </a-card>
      </a-grid-item>
    </a-grid>

    <a-grid :cols="24" :col-gap="16" :row-gap="16" class="section">
      <a-grid-item :span="{ xs: 24, lg: 16 }">
        <a-card title="最近值得看" :bordered="false">
          <a-table
            row-key="id"
            :columns="signalColumns"
            :data="highSignals"
            :loading="loading"
            :pagination="false"
          >
            <template #source="{ record }">{{ sourceLabel(record.source) }}</template>
            <template #priority="{ record }">
              <a-tag :color="priorityColor(record.priority)">{{ record.priority }}</a-tag>
            </template>
            <template #created_at="{ record }">{{ formatTime(record.created_at) }}</template>
          </a-table>
        </a-card>
      </a-grid-item>

      <a-grid-item :span="{ xs: 24, lg: 8 }">
        <a-card title="雷达模块" :bordered="false">
          <a-list :bordered="false">
            <a-list-item v-for="item in radarSources" :key="item.key">
              <a-list-item-meta :title="item.label" :description="item.focus" />
              <template #actions>
                <router-link :to="`/radars/${item.key}`">查看</router-link>
              </template>
            </a-list-item>
          </a-list>
        </a-card>
      </a-grid-item>
    </a-grid>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue';
  import type { TableColumnData } from '@arco-design/web-vue/es/table/interface';
  import {
    queryJobs,
    querySignals,
    queryWatchlist,
    ScannerRun,
    SignalEvent,
    WatchlistItem,
  } from '@/api/radar';
  import { formatTime, priorityColor, radarSources, sourceLabel } from '../shared';

  const loading = ref(false);
  const signals = ref<SignalEvent[]>([]);
  const jobs = ref<ScannerRun[]>([]);
  const watchlist = ref<WatchlistItem[]>([]);

  const highSignals = computed(() =>
    signals.value
      .filter((item) => item.priority === 'high' || item.priority === 'medium')
      .slice(0, 8)
  );

  const signalColumns: TableColumnData[] = [
    { title: '时间', dataIndex: 'created_at', slotName: 'created_at', width: 110 },
    { title: '来源', dataIndex: 'source', slotName: 'source', width: 130 },
    { title: '标的', dataIndex: 'symbol', width: 120 },
    { title: '类型', dataIndex: 'signal_type', width: 150 },
    { title: '优先级', dataIndex: 'priority', slotName: 'priority', width: 100 },
    { title: '说明', dataIndex: 'reason' },
  ];

  onMounted(async () => {
    loading.value = true;
    try {
      const [signalRes, jobRes, watchlistRes] = await Promise.all([
        querySignals({ time_range: '24h' }),
        queryJobs(),
        queryWatchlist(),
      ]);
      signals.value = signalRes.data?.items || [];
      jobs.value = jobRes.data?.items || [];
      watchlist.value = watchlistRes.data?.items || [];
    } finally {
      loading.value = false;
    }
  });
</script>

<style scoped lang="less">
  .radar-page {
    padding: 16px 20px;
  }

  .section {
    margin-top: 16px;
  }
</style>
