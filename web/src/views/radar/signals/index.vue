<template>
  <div class="radar-page">
    <a-page-header title="信号列表" subtitle="聚合 S1-S7 扫描器产生的信号" />
    <a-card :bordered="false">
      <a-space wrap class="filters">
        <a-select v-model="filters.source" placeholder="来源" allow-clear style="width: 180px">
          <a-option v-for="item in radarSources" :key="item.key" :value="item.key">
            {{ item.label }}
          </a-option>
        </a-select>
        <a-select v-model="filters.priority" placeholder="优先级" allow-clear style="width: 140px">
          <a-option value="high">high</a-option>
          <a-option value="medium">medium</a-option>
          <a-option value="low">low</a-option>
        </a-select>
        <a-select v-model="filters.time_range" style="width: 120px">
          <a-option value="1h">1h</a-option>
          <a-option value="6h">6h</a-option>
          <a-option value="24h">24h</a-option>
          <a-option value="7d">7d</a-option>
        </a-select>
        <a-button type="primary" @click="fetchData">刷新</a-button>
      </a-space>
      <a-table
        row-key="id"
        :columns="columns"
        :data="signals"
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
  import { onMounted, reactive, ref } from 'vue';
  import type { TableColumnData } from '@arco-design/web-vue/es/table/interface';
  import { querySignals, SignalEvent } from '@/api/radar';
  import { formatTime, priorityColor, radarSources, sourceLabel } from '../shared';

  const loading = ref(false);
  const signals = ref<SignalEvent[]>([]);
  const filters = reactive({
    source: '',
    priority: '',
    time_range: '24h',
  });

  const columns: TableColumnData[] = [
    { title: '时间', dataIndex: 'created_at', slotName: 'created_at', width: 110 },
    { title: '来源', dataIndex: 'source', slotName: 'source', width: 130 },
    { title: '链', dataIndex: 'chain', width: 90 },
    { title: '标的', dataIndex: 'symbol', width: 130 },
    { title: '类型', dataIndex: 'signal_type', width: 170 },
    { title: '优先级', dataIndex: 'priority', slotName: 'priority', width: 100 },
    { title: '分数', dataIndex: 'score', width: 90 },
    { title: '说明', dataIndex: 'reason' },
  ];

  async function fetchData() {
    loading.value = true;
    try {
      const res = await querySignals(filters);
      signals.value = res.data?.items || [];
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

  .filters {
    margin-bottom: 16px;
  }
</style>
