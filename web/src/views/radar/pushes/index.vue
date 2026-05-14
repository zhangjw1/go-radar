<template>
  <div class="radar-page">
    <a-page-header title="TG 推送" subtitle="已经推送到 Telegram 的信号记录" />
    <a-card :bordered="false">
      <a-button type="primary" class="toolbar" @click="fetchData">刷新</a-button>
      <a-table
        row-key="id"
        :columns="columns"
        :data="pushes"
        :loading="loading"
        :pagination="{ pageSize: 20 }"
      >
        <template #source="{ record }">{{ sourceLabel(record.source) }}</template>
        <template #priority="{ record }">
          <a-tag :color="priorityColor(record.priority)">{{ record.priority }}</a-tag>
        </template>
        <template #created_at="{ record }">{{ formatTime(record.created_at) }}</template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue';
  import type { TableColumnData } from '@arco-design/web-vue/es/table/interface';
  import { queryPushes, SignalEvent } from '@/api/radar';
  import { formatTime, priorityColor, sourceLabel } from '../shared';

  const loading = ref(false);
  const pushes = ref<SignalEvent[]>([]);
  const columns: TableColumnData[] = [
    { title: '时间', dataIndex: 'created_at', slotName: 'created_at', width: 110 },
    { title: '来源', dataIndex: 'source', slotName: 'source', width: 130 },
    { title: '标的', dataIndex: 'symbol', width: 130 },
    { title: '类型', dataIndex: 'signal_type', width: 170 },
    { title: '优先级', dataIndex: 'priority', slotName: 'priority', width: 100 },
    { title: '说明', dataIndex: 'reason' },
  ];

  async function fetchData() {
    loading.value = true;
    try {
      const res = await queryPushes({ time_range: '24h' });
      pushes.value = res.data?.items || [];
    } finally {
      loading.value = false;
    }
  }

  onMounted(fetchData);
</script>

<style scoped lang="less">
  .radar-page {
    padding: 16px 20px;
  }

  .toolbar {
    margin-bottom: 16px;
  }
</style>
