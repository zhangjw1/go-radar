<template>
  <aside
    class="w-full md:w-[320px] lg:w-[355px] shrink-0 border-l border-slate-200 h-full flex flex-col bg-white overflow-y-auto"
  >
    <!-- 盈亏分析 -->
    <div class="p-4 border-b border-slate-200 space-y-3">
      <div class="flex items-center justify-between">
        <span class="text-xs font-bold uppercase tracking-wider text-slate-800">
          盈亏分析
        </span>
        <div
          class="bg-slate-100 p-0.5 rounded-md border border-slate-200 flex text-[11px] font-semibold"
        >
          <button
            v-for="p in periods"
            :key="p.value"
            type="button"
            class="px-2 py-0.5 rounded cursor-pointer"
            :class="
              insiderStore.period === p.value
                ? 'bg-[#09182b] text-white'
                : 'text-slate-600 hover:text-slate-900'
            "
            @click="insiderStore.changePeriod(p.value)"
          >
            {{ p.label }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-2.5 text-xs">
        <div class="p-2.5 rounded-lg bg-slate-50 border border-slate-100">
          <div class="text-[10px] text-slate-400">胜率</div>
          <div class="text-base font-mono font-bold text-slate-900 mt-0.5">
            {{ Number(analytics?.win_rate || 0).toFixed(1) }}%
          </div>
        </div>
        <div class="p-2.5 rounded-lg bg-slate-50 border border-slate-100">
          <div class="text-[10px] text-slate-400">总盈亏</div>
          <div
            class="text-base font-mono font-bold mt-0.5"
            :class="
              (analytics?.total_pnl ?? 0) >= 0
                ? 'text-emerald-600'
                : 'text-rose-600'
            "
          >
            {{ formatUsd(analytics?.total_pnl, true) }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg bg-slate-50 border border-slate-100">
          <div class="text-[10px] text-slate-400">已实现盈亏</div>
          <div class="text-sm font-mono font-bold text-emerald-600 mt-0.5">
            {{ formatUsd(analytics?.realized_pnl, true) }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg bg-slate-50 border border-slate-100">
          <div class="text-[10px] text-slate-400">未实现盈亏</div>
          <div
            class="text-sm font-mono font-bold mt-0.5"
            :class="
              (analytics?.unrealized_pnl ?? 0) >= 0
                ? 'text-emerald-600'
                : 'text-rose-600'
            "
          >
            {{ formatUsd(analytics?.unrealized_pnl, true) }}
          </div>
        </div>
        <div class="p-2.5 rounded-lg bg-slate-50 border border-slate-100">
          <div class="text-[10px] text-slate-400">交易次数 (买/卖)</div>
          <div class="text-sm font-mono font-bold text-slate-900 mt-0.5">
            {{ analytics?.tx_count ?? 0 }}
            <span class="text-[10px] font-normal text-slate-400">
              ({{ analytics?.buy_count ?? 0 }}/{{ analytics?.sell_count ?? 0 }})
            </span>
          </div>
        </div>
        <div class="p-2.5 rounded-lg bg-slate-50 border border-slate-100">
          <div class="text-[10px] text-slate-400">总投入成本</div>
          <div class="text-sm font-mono font-bold text-slate-900 mt-0.5">
            {{ formatUsd(analytics?.total_cost) }}
          </div>
        </div>
      </div>
    </div>

    <!-- 告警历史 + 告警规则入口 -->
    <div class="p-4 flex-1 flex flex-col space-y-3">
      <div class="flex items-center justify-between">
        <span class="text-xs font-bold uppercase tracking-wider text-slate-800">
          告警历史 ({{ insiderStore.history.length }})
        </span>
        <button
          type="button"
          class="text-xs font-semibold text-blue-600 hover:underline cursor-pointer"
          @click="appStore.openSettings('alerts')"
        >
          告警规则 →
        </button>
      </div>

      <div
        class="bg-amber-50 border border-amber-200 text-amber-800 rounded-md px-2.5 py-1.5 text-[10.5px]"
      >
        钱包告警仅记录到历史表，不会主动向外发送。
      </div>

      <div class="space-y-2 flex-1 overflow-y-auto">
        <div
          v-if="insiderStore.history.length === 0"
          class="py-8 text-center text-xs text-slate-400"
        >
          暂无钱包告警记录
        </div>
        <div
          v-for="item in insiderStore.history"
          :key="item.id"
          class="p-2.5 rounded-lg border border-slate-200 bg-slate-50/70 text-[11px] space-y-1"
        >
          <div class="flex items-center justify-between">
            <span class="font-mono font-bold text-slate-800">
              {{ item.alert_type }}
            </span>
            <span
              class="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase"
              :class="
                item.level === 'warning'
                  ? 'bg-amber-100 text-amber-800'
                  : 'bg-blue-50 text-blue-700'
              "
            >
              {{ item.level }}
            </span>
          </div>
          <div class="text-slate-600 leading-snug">{{ item.message }}</div>
          <div class="text-[10px] font-mono text-slate-400">
            {{ formatTime(item.created_at) }}
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
  import { computed } from 'vue';
  import { useAppStore, useInsiderStore } from '@/store';
  import { formatTime, formatUsd } from '@/utils/format';

  const appStore = useAppStore();
  const insiderStore = useInsiderStore();

  const periods: Array<{ value: '7d' | '30d' | 'all'; label: string }> = [
    { value: '7d', label: '7d' },
    { value: '30d', label: '30d' },
    { value: 'all', label: '全部' },
  ];

  const analytics = computed(() => insiderStore.analytics);
</script>
