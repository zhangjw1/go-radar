<template>
  <div class="radar-page">
    <a-page-header title="任务状态" subtitle="查看扫描器最近运行结果" />
    <a-card :bordered="false">
      <a-button type="primary" class="toolbar" @click="fetchData">刷新</a-button>
      <a-table
        row-key="id"
        :columns="columns"
        :data="jobs"
        :loading="loading"
        :pagination="{ pageSize: 20 }"
      >
        <template #scanner="{ record }">{{ sourceLabel(record.scanner) }}</template>
        <template #status="{ record }">
          <a-tag :color="record.status === 'ok' ? 'green' : 'red'">{{ record.status }}</a-tag>
        </template>
        <template #started_at="{ record }">{{ formatTime(record.started_at) }}</template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue';
  import type { TableColumnData } from '@arco-design/web-vue/es/table/interface';
  import { queryJobs, ScannerRun } from '@/api/radar';
  import { formatTime, sourceLabel } from '../shared';

  const loading = ref(false);
  const jobs = ref<ScannerRun[]>([]);
  const columns: TableColumnData[] = [
    { title: '开始时间', dataIndex: 'started_at', slotName: 'started_at', width: 130 },
    { title: '雷达', dataIndex: 'scanner', slotName: 'scanner', width: 140 },
    { title: '状态', dataIndex: 'status', slotName: 'status', width: 100 },
    { title: '信号', dataIndex: 'signal_count', width: 90 },
    { title: '快照', dataIndex: 'snapshot_count', width: 90 },
    { title: '错误', dataIndex: 'error' },
  ];

  async function fetchData() {
    loading.value = true;
    try {
      const res = await queryJobs();
      jobs.value = res.data?.items || [];
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
