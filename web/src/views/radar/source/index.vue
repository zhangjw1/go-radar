<template>
  <div class="container">
    <Breadcrumb :items="['menu.list', meta.locale]" />
    <a-card class="general-card" :title="meta.label">
      <a-row>
        <a-col :flex="1">
          <a-form
            :model="formModel"
            :label-col-props="{ span: 6 }"
            :wrapper-col-props="{ span: 18 }"
            label-align="left"
          >
            <a-row :gutter="16">
              <a-col :span="8">
                <a-form-item field="symbol" label="标的">
                  <a-input
                    v-model="formModel.symbol"
                    placeholder="输入币种或合约"
                  />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item field="chain" label="链/市场">
                  <a-select
                    v-model="formModel.chain"
                    :options="chainOptions"
                    placeholder="全部链/市场"
                    allow-clear
                  />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item field="signal_type" label="信号类型">
                  <a-select
                    v-model="formModel.signal_type"
                    :options="signalTypeOptions"
                    placeholder="全部信号类型"
                    allow-clear
                  />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item field="priority" label="优先级">
                  <a-select
                    v-model="formModel.priority"
                    :options="priorityOptions"
                    placeholder="全部"
                    allow-clear
                  />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item field="time_range" label="时间范围">
                  <a-select
                    v-model="formModel.time_range"
                    :options="timeRangeOptions"
                    placeholder="全部"
                  />
                </a-form-item>
              </a-col>
              <a-col :span="8">
                <a-form-item field="watchlist_only" label="观察名单">
                  <a-select
                    v-model="formModel.watchlist_only"
                    :options="watchlistOptions"
                    placeholder="全部"
                  />
                </a-form-item>
              </a-col>
            </a-row>
          </a-form>
        </a-col>
        <a-divider style="height: 84px" direction="vertical" />
        <a-col :flex="'86px'" style="text-align: right">
          <a-space direction="vertical" :size="18">
            <a-button type="primary" @click="search">
              <template #icon>
                <icon-search />
              </template>
              查询
            </a-button>
            <a-button @click="reset">
              <template #icon>
                <icon-refresh />
              </template>
              重置
            </a-button>
          </a-space>
        </a-col>
      </a-row>
      <a-divider style="margin-top: 0" />
      <a-row style="margin-bottom: 16px">
        <a-col :span="12">
          <a-space>
            <a-button type="primary" @click="search">
              <template #icon>
                <icon-refresh />
              </template>
              刷新
            </a-button>
            <a-tag color="arcoblue">{{ meta.focus }}</a-tag>
          </a-space>
        </a-col>
        <a-col
          :span="12"
          style="display: flex; align-items: center; justify-content: end"
        >
          <a-button @click="downloadCsv">
            <template #icon>
              <icon-download />
            </template>
            下载
          </a-button>
          <a-tooltip content="刷新">
            <div class="action-icon" @click="search">
              <icon-refresh size="18" />
            </div>
          </a-tooltip>
          <a-dropdown @select="handleSelectDensity">
            <a-tooltip content="密度">
              <div class="action-icon"><icon-line-height size="18" /></div>
            </a-tooltip>
            <template #content>
              <a-doption
                v-for="item in densityList"
                :key="item.value"
                :value="item.value"
                :class="{ active: item.value === size }"
              >
                <span>{{ item.name }}</span>
              </a-doption>
            </template>
          </a-dropdown>
          <a-tooltip content="列设置">
            <a-popover
              trigger="click"
              position="bl"
              @popup-visible-change="popupVisibleChange"
            >
              <div class="action-icon"><icon-settings size="18" /></div>
              <template #content>
                <div id="radarTableSetting">
                  <div
                    v-for="(item, index) in showColumns"
                    :key="item.dataIndex"
                    class="setting"
                  >
                    <div style="margin-right: 4px; cursor: move">
                      <icon-drag-arrow />
                    </div>
                    <div>
                      <a-checkbox
                        v-model="item.checked"
                        @change="
                          handleChange($event, item as TableColumnData, index)
                        "
                      >
                      </a-checkbox>
                    </div>
                    <div class="title">
                      {{ item.title === '#' ? '序号' : item.title }}
                    </div>
                  </div>
                </div>
              </template>
            </a-popover>
          </a-tooltip>
        </a-col>
      </a-row>
      <a-table
        row-key="id"
        :loading="loading"
        :pagination="pagination"
        :columns="(cloneColumns as TableColumnData[])"
        :data="renderData"
        :bordered="false"
        :size="size"
        @page-change="onPageChange"
      >
        <template #index="{ rowIndex }">
          {{ rowIndex + 1 + (pagination.current - 1) * pagination.pageSize }}
        </template>
        <template #symbol="{ record }">
          <a-space direction="vertical" :size="0">
            <a-link @click="goTokenDetail(record)">{{
              record.symbol || '-'
            }}</a-link>
            <span class="table-sub-text">{{
              shortAddress(record.address)
            }}</span>
          </a-space>
        </template>
        <template #chain="{ record }">
          <a-space direction="vertical" :size="0">
            <span>{{ chainLabel(record.chain) }}</span>
            <span class="table-sub-text">{{ record.chain }}</span>
          </a-space>
        </template>
        <template #signal_type="{ record }">
          <a-space direction="vertical" :size="0">
            <span class="signal-type-text">{{
              signalTypeLabel(record.signal_type)
            }}</span>
            <span class="table-sub-text">{{ record.signal_type }}</span>
          </a-space>
        </template>
        <template #priority="{ record }">
          <a-tag :color="priorityColor(record.priority)">
            {{ priorityLabel(record.priority) }}
          </a-tag>
        </template>
        <template #score="{ record }">
          <div class="score-cell" :class="scoreLevel(record.score)">
            <div class="score-value">{{ formatScore(record.score) }}</div>
            <div class="score-text">{{ scoreText(record.score) }}</div>
          </div>
        </template>
        <template #reason="{ record }">
          <a-space direction="vertical" :size="0">
            <span class="reason-text">{{
              summarizeReason(record.signal_type, record.reason)
            }}</span>
            <span class="table-sub-text">{{ record.reason }}</span>
          </a-space>
        </template>
        <template #created_at="{ record }">
          {{ formatTime(record.created_at) }}
        </template>
        <template #pushed_at="{ record }">
          <span v-if="record.pushed_at" class="circle pass"></span>
          <span v-else class="circle"></span>
          {{ record.pushed_at ? '已推送' : '未推送' }}
        </template>
        <template #operations="{ record }">
          <a-space>
            <a-button type="text" size="small" @click="goTokenDetail(record)">
              详情
            </a-button>
            <a-button type="text" size="small" @click="copyAddress(record)">
              复制地址
            </a-button>
          </a-space>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
  import { computed, nextTick, reactive, ref, watch } from 'vue';
  import { useRouter } from 'vue-router';
  import { Message } from '@arco-design/web-vue';
  import type { SelectOptionData } from '@arco-design/web-vue/es/select/interface';
  import type { TableColumnData } from '@arco-design/web-vue/es/table/interface';
  import cloneDeep from 'lodash/cloneDeep';
  import Sortable from 'sortablejs';
  import type { Pagination } from '@/types/global';
  import {
    querySignals,
    type FilterOptions,
    type SignalEvent,
  } from '@/api/radar';
  import {
    chainLabel,
    formatScore,
    formatTime,
    priorityColor,
    priorityLabel,
    radarSources,
    scoreLevel,
    scoreText,
    signalTypeLabel,
    summarizeReason,
  } from '../shared';

  type SizeProps = 'mini' | 'small' | 'medium' | 'large';
  type Column = TableColumnData & { checked?: true };

  const props = defineProps<{ source: string }>();
  const router = useRouter();

  const sourceLocales: Record<string, string> = {
    s1: 'menu.radar.s1',
    s2: 'menu.radar.s2',
    s3: 'menu.radar.s3',
    s5: 'menu.radar.s5',
    s7: 'menu.radar.s7',
  };

  const meta = computed(() => {
    const current =
      radarSources.find((item) => item.key === props.source) || radarSources[0];
    return {
      ...current,
      locale: sourceLocales[current.key] || 'menu.radar.signals',
    };
  });

  const generateFormModel = () => ({
    symbol: '',
    chain: '',
    signal_type: '',
    priority: '',
    time_range: '24h',
    watchlist_only: '',
  });

  const loading = ref(false);
  const renderData = ref<SignalEvent[]>([]);
  const formModel = ref(generateFormModel());
  const cloneColumns = ref<Column[]>([]);
  const showColumns = ref<Column[]>([]);
  const size = ref<SizeProps>('medium');
  const filterOptions = ref<FilterOptions>({
    sources: [],
    chains: [],
    signalTypes: [],
    priorities: ['high', 'medium', 'low'],
    timeRanges: ['1h', '6h', '24h', '7d'],
  });

  const basePagination: Pagination = {
    current: 1,
    pageSize: 20,
  };

  const pagination = reactive({
    ...basePagination,
  });

  const densityList = [
    { name: '迷你', value: 'mini' },
    { name: '偏小', value: 'small' },
    { name: '中等', value: 'medium' },
    { name: '偏大', value: 'large' },
  ];

  const priorityOptions = computed<SelectOptionData[]>(() =>
    filterOptions.value.priorities.map((value) => ({
      label: priorityLabel(value),
      value,
    }))
  );

  const timeRangeLabel = (value: string) => {
    if (value === '1h') return '最近 1 小时';
    if (value === '6h') return '最近 6 小时';
    if (value === '24h') return '最近 24 小时';
    if (value === '7d') return '最近 7 天';
    return value;
  };

  const timeRangeOptions = computed<SelectOptionData[]>(() =>
    filterOptions.value.timeRanges.map((value) => ({
      label: timeRangeLabel(value),
      value,
    }))
  );

  const chainOptions = computed<SelectOptionData[]>(() =>
    filterOptions.value.chains.map((value) => ({
      label: chainLabel(value),
      value,
    }))
  );

  const signalTypeOptions = computed<SelectOptionData[]>(() =>
    filterOptions.value.signalTypes.map((value) => ({
      label: signalTypeLabel(value),
      value,
    }))
  );

  const watchlistOptions: SelectOptionData[] = [
    { label: '全部', value: '' },
    { label: '只看观察名单', value: 'true' },
  ];

  const columns = computed<TableColumnData[]>(() => [
    {
      title: '#',
      dataIndex: 'index',
      slotName: 'index',
      width: 70,
    },
    {
      title: '标的',
      dataIndex: 'symbol',
      slotName: 'symbol',
      width: 180,
    },
    {
      title: '链/市场',
      dataIndex: 'chain',
      slotName: 'chain',
      width: 130,
    },
    {
      title: '信号类型',
      dataIndex: 'signal_type',
      slotName: 'signal_type',
      width: 190,
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      slotName: 'priority',
      width: 110,
    },
    {
      title: '分数',
      dataIndex: 'score',
      slotName: 'score',
      width: 120,
    },
    {
      title: '说明',
      dataIndex: 'reason',
      slotName: 'reason',
      width: 320,
      ellipsis: true,
      tooltip: true,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      slotName: 'created_at',
      width: 150,
    },
    {
      title: '推送状态',
      dataIndex: 'pushed_at',
      slotName: 'pushed_at',
      width: 120,
    },
    {
      title: '操作',
      dataIndex: 'operations',
      slotName: 'operations',
      width: 120,
    },
  ]);

  const shortAddress = (value?: string) => {
    if (!value) return '-';
    if (value.length <= 14) return value;
    return `${value.slice(0, 6)}...${value.slice(-6)}`;
  };

  const buildParams = (current = pagination.current) => {
    const params: Record<string, unknown> = {
      source: props.source,
      time_range: formModel.value.time_range,
      page: current,
      pageSize: pagination.pageSize,
    };
    if (formModel.value.symbol) params.symbol = formModel.value.symbol;
    if (formModel.value.chain) params.chain = formModel.value.chain;
    if (formModel.value.signal_type) {
      params.signal_type = formModel.value.signal_type;
    }
    if (formModel.value.priority) params.priority = formModel.value.priority;
    if (formModel.value.watchlist_only) {
      params.watchlist_only = formModel.value.watchlist_only;
    }
    return params;
  };

  async function fetchData(current = 1) {
    loading.value = true;
    try {
      const res = await querySignals(buildParams(current));
      renderData.value = res.data?.items || [];
      pagination.current = res.data?.current || current;
      pagination.pageSize = res.data?.pageSize || pagination.pageSize;
      pagination.total = res.data?.total || 0;
      if (res.data?.filters) {
        filterOptions.value = res.data.filters;
      }
    } catch (err) {
      renderData.value = [];
      pagination.current = current;
      pagination.total = 0;
      Message.error('信号数据加载失败');
    } finally {
      loading.value = false;
    }
  }

  const search = () => {
    fetchData(1);
  };

  const reset = () => {
    formModel.value = generateFormModel();
    fetchData(1);
  };

  const onPageChange = (current: number) => {
    fetchData(current);
  };

  const handleSelectDensity = (
    val: string | number | Record<string, any> | undefined
  ) => {
    size.value = val as SizeProps;
  };

  const handleChange = (
    checked: boolean | (string | boolean | number)[],
    column: Column,
    index: number
  ) => {
    if (!checked) {
      cloneColumns.value = showColumns.value.filter(
        (item) => item.dataIndex !== column.dataIndex
      );
    } else {
      cloneColumns.value.splice(index, 0, column);
    }
  };

  const exchangeArray = <T extends Array<any>>(
    array: T,
    beforeIdx: number,
    newIdx: number,
    isDeep = false
  ): T => {
    const newArray = isDeep ? cloneDeep(array) : array;
    if (beforeIdx > -1 && newIdx > -1) {
      newArray.splice(
        beforeIdx,
        1,
        newArray.splice(newIdx, 1, newArray[beforeIdx]).pop()
      );
    }
    return newArray;
  };

  const popupVisibleChange = (val: boolean) => {
    if (val) {
      nextTick(() => {
        const el = document.getElementById('radarTableSetting') as HTMLElement;
        if (!el) return;
        // eslint-disable-next-line no-new
        new Sortable(el, {
          onEnd(e: any) {
            const { oldIndex, newIndex } = e;
            exchangeArray(cloneColumns.value, oldIndex, newIndex);
            exchangeArray(showColumns.value, oldIndex, newIndex);
          },
        });
      });
    }
  };

  const copyAddress = async (record: SignalEvent) => {
    if (!record.address) {
      Message.warning('暂无地址');
      return;
    }
    await navigator.clipboard.writeText(record.address);
    Message.success('地址已复制');
  };

  const goTokenDetail = (record: SignalEvent) => {
    if (!record.chain || !record.address) {
      Message.warning('暂无详情地址');
      return;
    }
    router.push({
      name: 'RadarTokenDetail',
      params: {
        chain: record.chain,
        address: record.address,
      },
    });
  };

  const downloadCsv = () => {
    const header = [
      '标的',
      '地址',
      '链/市场',
      '信号类型',
      '优先级',
      '分数',
      '说明',
      '创建时间',
    ];
    const rows = renderData.value.map((item) => [
      item.symbol,
      item.address,
      chainLabel(item.chain),
      signalTypeLabel(item.signal_type),
      priorityLabel(item.priority),
      formatScore(item.score),
      summarizeReason(item.signal_type, item.reason),
      formatTime(item.created_at),
    ]);
    const csv = [header, ...rows]
      .map((row) =>
        row
          .map((cell) => `"${String(cell || '').replace(/"/g, '""')}"`)
          .join(',')
      )
      .join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${props.source}-signals.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  watch(
    () => columns.value,
    (val) => {
      cloneColumns.value = cloneDeep(val);
      cloneColumns.value.forEach((item) => {
        item.checked = true;
      });
      showColumns.value = cloneDeep(cloneColumns.value);
    },
    { deep: true, immediate: true }
  );

  watch(
    () => props.source,
    () => {
      formModel.value = generateFormModel();
      fetchData(1);
    },
    { immediate: true }
  );
</script>

<script lang="ts">
  export default {
    name: 'RadarSourceList',
  };
</script>

<style scoped lang="less">
  .container {
    padding: 0 20px 20px 20px;
  }

  .action-icon {
    margin-left: 12px;
    cursor: pointer;
  }

  .active {
    color: #0960bd;
    background-color: #e3f4fc;
  }

  .setting {
    display: flex;
    align-items: center;
    width: 200px;

    .title {
      margin-left: 12px;
      cursor: pointer;
    }
  }

  .table-sub-text {
    color: var(--color-text-3);
    font-size: 12px;
  }

  .signal-type-text,
  .reason-text {
    line-height: 20px;
  }

  .score-cell {
    min-width: 84px;
    padding: 8px 10px;
    border-radius: 6px;
    text-align: center;
    background: var(--color-fill-2);
  }

  .score-value {
    font-size: 16px;
    font-weight: 700;
    line-height: 20px;
  }

  .score-text {
    margin-top: 2px;
    font-size: 12px;
    color: var(--color-text-3);
    line-height: 16px;
  }

  .score-cell.critical {
    background: #ffece8;
    box-shadow: inset 0 0 0 1px rgba(245, 63, 63, 0.2);
  }

  .score-cell.critical .score-value {
    color: rgb(var(--red-6));
  }

  .score-cell.hot {
    background: #fff3e8;
    box-shadow: inset 0 0 0 1px rgba(255, 125, 0, 0.2);
  }

  .score-cell.hot .score-value {
    color: rgb(var(--orange-6));
  }

  .score-cell.warm {
    background: #f5f9ff;
    box-shadow: inset 0 0 0 1px rgba(22, 93, 255, 0.16);
  }

  .score-cell.warm .score-value {
    color: rgb(var(--arcoblue-6));
  }

  .circle {
    display: inline-block;
    width: 6px;
    height: 6px;
    margin-right: 8px;
    vertical-align: 1px;
    background-color: rgb(var(--gray-5));
    border-radius: 50%;

    &.pass {
      background-color: rgb(var(--green-6));
    }
  }
</style>
