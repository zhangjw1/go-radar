<template>
  <div
    class="flex-1 bg-white border border-slate-200 rounded-xl flex flex-col min-h-0 overflow-hidden"
  >
    <!-- 顶部 Tab 与筛选栏 -->
    <div
      class="px-4 py-3 border-b border-slate-100 flex flex-wrap items-center justify-between gap-2.5 shrink-0"
    >
      <div class="flex items-center gap-2">
        <div
          class="bg-slate-100 p-0.5 rounded-lg border border-slate-200/80 flex items-center text-xs font-semibold"
        >
          <button
            type="button"
            class="px-3 py-1 rounded-md transition-all cursor-pointer"
            :class="
              radarStore.feedMode === 'signals'
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-500 hover:text-slate-800'
            "
            @click="$emit('changeFeedMode', 'signals')"
          >
            全部信号
          </button>
          <button
            type="button"
            class="px-3 py-1 rounded-md transition-all cursor-pointer"
            :class="
              radarStore.feedMode === 'pushes'
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-500 hover:text-slate-800'
            "
            @click="$emit('changeFeedMode', 'pushes')"
          >
            已推送 TG
          </button>
        </div>

        <span
          v-if="radarStore.filters.source"
          class="text-[11px] font-medium px-2 py-0.5 rounded bg-blue-50 text-blue-700 border border-blue-200 flex items-center gap-1"
        >
          <span>来源: {{ sourceLabel(radarStore.filters.source) }}</span>
          <button
            type="button"
            class="hover:text-blue-900 cursor-pointer"
            @click="$emit('clearSource')"
          >
            &times;
          </button>
        </span>
      </div>

      <!-- 动态筛选器（选项全部从后端 filters 读取） -->
      <div class="flex items-center gap-2 text-xs flex-wrap">
        <input
          v-model="symbolKeyword"
          type="text"
          placeholder="搜索标的 Symbol..."
          class="w-36 px-2.5 py-1.5 border border-slate-200 rounded-md text-xs focus:outline-none focus:border-blue-500"
          @keydown.enter="applySymbolFilter()"
          @blur="applySymbolFilter()"
        />

        <select
          :value="radarStore.filters.chain"
          class="px-2 py-1.5 border border-slate-200 rounded-md bg-white text-slate-700 font-medium focus:outline-none focus:border-blue-500 cursor-pointer"
          @change="onFilterChange('chain', $event)"
        >
          <option value="">全部链/市场</option>
          <option
            v-for="ch in radarStore.filterOptions.chains"
            :key="ch"
            :value="ch"
          >
            {{ chainLabel(ch) }}
          </option>
        </select>

        <select
          :value="radarStore.filters.signal_type"
          class="px-2 py-1.5 border border-slate-200 rounded-md bg-white text-slate-700 font-medium focus:outline-none focus:border-blue-500 cursor-pointer"
          @change="onFilterChange('signal_type', $event)"
        >
          <option value="">全部信号类型</option>
          <option
            v-for="st in radarStore.filterOptions.signalTypes"
            :key="st"
            :value="st"
          >
            {{ signalTypeLabel(st) }}
          </option>
        </select>

        <select
          :value="radarStore.filters.priority"
          class="px-2 py-1.5 border border-slate-200 rounded-md bg-white text-slate-700 font-medium focus:outline-none focus:border-blue-500 cursor-pointer"
          @change="onFilterChange('priority', $event)"
        >
          <option value="">优先级: 全部</option>
          <option
            v-for="pr in radarStore.filterOptions.priorities"
            :key="pr"
            :value="pr"
          >
            {{ priorityLabel(pr) }}
          </option>
        </select>

        <select
          :value="radarStore.filters.time_range"
          class="px-2 py-1.5 border border-slate-200 rounded-md bg-white text-slate-700 font-medium focus:outline-none focus:border-blue-500 cursor-pointer"
          @change="onFilterChange('time_range', $event)"
        >
          <option
            v-for="tr in radarStore.filterOptions.timeRanges"
            :key="tr"
            :value="tr"
          >
            {{ TIME_RANGE_LABELS[tr] || tr }}
          </option>
        </select>

        <button
          type="button"
          class="px-2.5 py-1.5 border rounded-md font-medium transition-colors cursor-pointer"
          :class="
            radarStore.filters.watchlist_only === 'true'
              ? 'border-blue-600 bg-blue-600 text-white'
              : 'border-slate-200 text-slate-600 hover:bg-slate-50'
          "
          @click="$emit('toggleWatchlistOnly')"
        >
          ★ 只看观察
        </button>

        <button
          type="button"
          class="px-2.5 py-1.5 border border-slate-200 text-slate-600 hover:bg-slate-50 rounded-md font-medium cursor-pointer"
          @click="handleExportCsv()"
        >
          导出 CSV
        </button>
      </div>
    </div>

    <!-- 信号列表表格 -->
    <div class="flex-1 overflow-x-auto overflow-y-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead class="sticky top-0 bg-slate-50/95 z-10">
          <tr
            class="border-b border-slate-100 text-[11px] font-semibold text-slate-400"
          >
            <th class="py-2.5 px-4 font-medium w-20">优先级</th>
            <th class="py-2.5 px-4 font-medium">标的 / 地址</th>
            <th class="py-2.5 px-4 font-medium">链 / 市场</th>
            <th class="py-2.5 px-4 font-medium">来源 & 类型</th>
            <th class="py-2.5 px-4 font-medium">触发说明</th>
            <th class="py-2.5 px-4 font-medium">评分 / 推送</th>
            <th class="py-2.5 px-4 font-medium text-right">时间 / 报文</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-if="radarStore.loadingFeed && radarStore.signals.length === 0">
            <td colspan="7" class="py-12 text-center text-slate-400">
              正在加载信号数据...
            </td>
          </tr>
          <tr v-else-if="radarStore.signals.length === 0">
            <td colspan="7" class="py-12 text-center text-slate-400">
              当前筛选条件下暂无异动信号
            </td>
          </tr>
          <tr
            v-for="item in radarStore.signals"
            :key="item.id || `${item.chain}:${item.address}:${item.created_at}`"
            class="transition-colors cursor-pointer text-slate-800"
            :class="
              radarStore.selectedTokenKey === `${item.chain}:${item.address}`
                ? 'bg-blue-50/50'
                : 'hover:bg-slate-50/80'
            "
            @click="$emit('selectToken', item.chain, item.address)"
          >
            <td class="py-3 px-4 whitespace-nowrap">
              <span
                v-if="item.signal_type === 'resonance'"
                class="text-blue-600 font-bold"
              >
                ⚡ 共振
              </span>
              <span
                v-else
                class="font-bold uppercase text-[11px]"
                :class="
                  item.priority === 'high'
                    ? 'text-rose-600'
                    : item.priority === 'medium'
                    ? 'text-amber-600'
                    : 'text-slate-500'
                "
              >
                {{ item.priority }}
              </span>
            </td>
            <td class="py-3 px-4 whitespace-nowrap">
              <div class="font-bold text-slate-900">
                {{ item.symbol || shortAddress(item.address) }}
              </div>
              <div class="text-[10.5px] font-mono text-slate-400">
                {{ shortAddress(item.address) }}
              </div>
            </td>
            <td class="py-3 px-4 whitespace-nowrap">
              <span
                class="bg-slate-100 text-slate-700 px-1.5 py-0.5 rounded text-[11px] font-medium border border-slate-200"
              >
                {{ chainLabel(item.chain) }}
              </span>
            </td>
            <td class="py-3 px-4 whitespace-nowrap">
              <div class="font-semibold text-slate-800">
                {{
                  signalTypeLabel(item.signal_type, item.reason, item.raw_json)
                }}
              </div>
              <div class="text-[10.5px] font-mono text-slate-400">
                {{ sourceLabel(item.source) }}
              </div>
            </td>
            <td class="py-3 px-4 max-w-md">
              <div class="text-slate-800 font-medium leading-snug line-clamp-2">
                {{ summarizeReason(item.signal_type, item.reason) }}
              </div>
            </td>
            <td class="py-3 px-4 whitespace-nowrap">
              <div class="w-28 space-y-1">
                <div
                  class="flex items-center justify-between text-[11px] font-mono"
                >
                  <span
                    class="font-bold"
                    :class="
                      item.score >= 85 ? 'text-rose-600' : 'text-blue-600'
                    "
                  >
                    {{ Number(item.score || 0).toFixed(0) }} 分
                  </span>
                  <span
                    :class="
                      item.pushed_at
                        ? 'text-emerald-600 font-sans font-semibold'
                        : 'text-slate-400 font-sans'
                    "
                  >
                    {{ item.pushed_at ? '✓ 已推送' : '未推送' }}
                  </span>
                </div>
                <div
                  class="w-full bg-slate-100 rounded-full h-1 overflow-hidden"
                >
                  <div
                    class="h-1 rounded-full"
                    :class="item.score >= 85 ? 'bg-rose-500' : 'bg-blue-600'"
                    :style="{
                      width: `${Math.min(100, Math.max(0, item.score || 0))}%`,
                    }"
                  />
                </div>
              </div>
            </td>
            <td class="py-3 px-4 text-right whitespace-nowrap">
              <div class="font-mono text-slate-400 text-[11px]">
                {{ formatTime(item.created_at) }}
              </div>
              <button
                type="button"
                class="text-[11px] text-blue-600 hover:underline mt-0.5 cursor-pointer"
                @click.stop="
                  appStore.openRawModal(
                    `${item.symbol} · 信号报文`,
                    item.source.toUpperCase(),
                    item.raw_json
                  )
                "
              >
                报文
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 服务端分页栏 -->
    <div
      class="px-4 py-2.5 border-t border-slate-100 flex items-center justify-between text-[11px] text-slate-500 shrink-0 bg-white"
    >
      <div class="flex items-center gap-2">
        <span
          >共 <strong class="font-mono text-slate-800">{{ total }}</strong> 条 ·
          第 <strong class="font-mono text-slate-800">{{ page }}</strong> /
          {{ totalPages }} 页</span
        >
        <select
          :value="radarStore.filters.pageSize"
          class="border border-slate-200 rounded px-1.5 py-0.5 bg-white font-mono text-[11px]"
          @change="onPageSizeChange($event)"
        >
          <option :value="20">20 条/页</option>
          <option :value="50">50 条/页</option>
        </select>
      </div>
      <div class="flex items-center gap-1.5">
        <button
          type="button"
          :disabled="page <= 1"
          class="px-2.5 py-1 border border-slate-200 rounded hover:bg-slate-50 disabled:opacity-40 cursor-pointer"
          @click="$emit('changePage', page - 1)"
        >
          上一页
        </button>
        <button
          type="button"
          :disabled="page >= totalPages"
          class="px-2.5 py-1 border border-slate-200 rounded hover:bg-slate-50 disabled:opacity-40 cursor-pointer"
          @click="$emit('changePage', page + 1)"
        >
          下一页
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { useAppStore, useRadarStore, type FeedMode } from '@/store';
  import { TIME_RANGE_LABELS } from '@/constants/radar';
  import {
    chainLabel,
    exportSignalsToCsv,
    formatTime,
    priorityLabel,
    shortAddress,
    signalTypeLabel,
    sourceLabel,
    summarizeReason,
  } from '@/utils/format';

  const emit = defineEmits<{
    (e: 'changeFeedMode', mode: FeedMode): void;
    (e: 'updateFilter', key: string, value: string): void;
    (e: 'clearSource'): void;
    (e: 'toggleWatchlistOnly'): void;
    (e: 'changePage', page: number): void;
    (e: 'changePageSize', size: number): void;
    (e: 'selectToken', chain: string, address: string): void;
  }>();

  const appStore = useAppStore();
  const radarStore = useRadarStore();
  const symbolKeyword = ref(radarStore.filters.symbol);

  watch(
    () => radarStore.filters.symbol,
    (val) => {
      symbolKeyword.value = val;
    }
  );

  const page = computed(() => radarStore.filters.page);
  const total = computed(() => radarStore.total);
  const totalPages = computed(() =>
    Math.max(1, Math.ceil(total.value / radarStore.filters.pageSize))
  );

  const applySymbolFilter = () => {
    const clean = symbolKeyword.value.trim();
    if (clean !== radarStore.filters.symbol) {
      emit('updateFilter', 'symbol', clean);
    }
  };

  const onFilterChange = (key: string, event: Event) => {
    emit('updateFilter', key, (event.target as HTMLSelectElement).value);
  };

  const onPageSizeChange = (event: Event) => {
    emit('changePageSize', Number((event.target as HTMLSelectElement).value));
  };

  const handleExportCsv = async () => {
    try {
      const items = await radarStore.exportCurrentCsv();
      exportSignalsToCsv(items);
      appStore.showToast(`已导出 ${items.length} 条信号到 CSV`, 'success');
    } catch {
      appStore.showToast('导出 CSV 失败', 'error');
    }
  };
</script>
