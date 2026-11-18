<template>
  <aside
    class="w-full md:w-[250px] lg:w-[268px] shrink-0 border-r border-slate-200 h-full flex flex-col bg-white overflow-hidden"
  >
    <!-- 上半区：雷达来源 -->
    <div
      class="px-3.5 py-2.5 border-b border-slate-100 flex items-center justify-between shrink-0"
    >
      <div class="flex items-center gap-2 text-slate-800">
        <svg
          class="w-3.5 h-3.5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.2"
        >
          <circle cx="12" cy="12" r="9" />
          <circle cx="12" cy="12" r="4" />
          <line x1="12" y1="3" x2="12" y2="5" />
        </svg>
        <span class="text-[11px] font-bold tracking-wider uppercase"
          >雷达来源</span
        >
      </div>
      <span class="text-[10px] font-mono text-slate-400">S1-S7</span>
    </div>

    <div class="p-2 space-y-0.5 border-b border-slate-200 shrink-0">
      <button
        type="button"
        class="w-full text-left px-2.5 py-1.5 rounded-md flex items-center justify-between transition-colors cursor-pointer"
        :class="
          radarStore.filters.source === ''
            ? 'bg-slate-100 text-slate-900'
            : 'text-slate-600 hover:bg-slate-50'
        "
        @click="$emit('selectSource', '')"
      >
        <div class="min-w-0">
          <div
            class="text-xs truncate"
            :class="
              radarStore.filters.source === ''
                ? 'font-bold text-slate-900'
                : 'font-medium'
            "
          >
            全部来源
          </div>
          <div class="text-[10px] text-slate-400 truncate">
            聚合所有扫描器信号
          </div>
        </div>
        <span
          class="text-[10px] font-mono font-bold px-1.5 py-0.5 rounded"
          :class="
            radarStore.filters.source === ''
              ? 'bg-[#09182b] text-white'
              : 'bg-slate-100 text-slate-500'
          "
        >
          {{ radarStore.stats.total24h }}
        </span>
      </button>

      <button
        v-for="sc in RADAR_SOURCES"
        :key="sc.key"
        type="button"
        class="w-full text-left px-2.5 py-1.5 rounded-md flex items-center justify-between transition-colors cursor-pointer"
        :class="
          radarStore.filters.source === sc.key
            ? 'bg-slate-100 text-slate-900'
            : 'text-slate-600 hover:bg-slate-50'
        "
        @click="$emit('selectSource', sc.key)"
      >
        <div class="min-w-0 flex items-center gap-2">
          <span
            class="w-1.5 h-1.5 rounded-full shrink-0"
            :class="statusDotClass(sc.key)"
            :title="`最近运行状态: ${latestStatus(sc.key)}`"
          />
          <div class="min-w-0">
            <div
              class="text-xs truncate"
              :class="
                radarStore.filters.source === sc.key
                  ? 'font-bold text-slate-900'
                  : 'font-medium'
              "
            >
              {{ sc.label }}
            </div>
            <div class="text-[10px] text-slate-400 truncate">
              {{ sc.focus }}
            </div>
          </div>
        </div>
        <span
          class="text-[10px] font-mono font-bold px-1.5 py-0.5 rounded"
          :class="
            radarStore.filters.source === sc.key
              ? 'bg-[#09182b] text-white'
              : 'bg-slate-100 text-slate-500'
          "
        >
          {{ radarStore.sourceCounts[sc.key] ?? 0 }}
        </span>
      </button>
    </div>

    <!-- 下半区：观察名单 -->
    <div
      class="px-3.5 py-2.5 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between shrink-0"
    >
      <span
        class="text-[11px] font-bold tracking-wider uppercase text-slate-800"
      >
        观察名单 ({{ radarStore.watchlist.length }})
      </span>
      <button
        type="button"
        class="px-2 py-0.5 rounded text-[11px] font-semibold border border-slate-200 bg-white hover:bg-slate-100 text-slate-700 cursor-pointer"
        @click="addFormOpen = !addFormOpen"
      >
        {{ addFormOpen ? '收起' : '+ 添加' }}
      </button>
    </div>

    <!-- 展开的添加表单（出现在名单上方） -->
    <form
      v-if="addFormOpen"
      class="p-3 border-b border-slate-200 bg-slate-50 space-y-2 text-xs shrink-0"
      @submit.prevent="submitAddWatch()"
    >
      <select
        v-model="form.chain"
        class="w-full border border-slate-200 rounded-md px-2 py-1.5 bg-white text-xs"
      >
        <option
          v-for="opt in WATCHLIST_CHAIN_OPTIONS"
          :key="opt.value"
          :value="opt.value"
        >
          {{ opt.label }}
        </option>
      </select>
      <input
        v-model="form.address"
        type="text"
        required
        placeholder="合约地址或交易对 (如 btcusdt)"
        class="w-full border border-slate-200 rounded-md px-2.5 py-1.5 font-mono text-[11px] bg-white focus:outline-none focus:border-blue-500"
      />
      <input
        v-model="form.note"
        type="text"
        placeholder="备注（可选）"
        class="w-full border border-slate-200 rounded-md px-2.5 py-1.5 text-xs bg-white focus:outline-none focus:border-blue-500"
      />
      <button
        type="submit"
        :disabled="submitting || !form.address.trim()"
        class="w-full bg-[#09182b] hover:bg-slate-800 disabled:bg-slate-300 text-white font-semibold py-1.5 rounded-md cursor-pointer"
      >
        {{ submitting ? '提交中...' : '确认添加' }}
      </button>
    </form>

    <!-- 观察名单列表 -->
    <div class="flex-1 overflow-y-auto divide-y divide-slate-100">
      <div
        v-if="radarStore.watchlist.length === 0"
        class="p-6 text-center text-xs text-slate-400"
      >
        暂无观察标的，点击上方「+ 添加」
      </div>
      <div
        v-for="item in radarStore.watchlist"
        :key="`${item.chain}:${item.address}`"
        class="py-2.5 px-3 transition-colors cursor-pointer group"
        :class="
          radarStore.selectedTokenKey === `${item.chain}:${item.address}`
            ? 'bg-blue-50/60'
            : 'hover:bg-slate-50/80'
        "
        @click="$emit('selectToken', item.chain, item.address)"
      >
        <div class="flex items-center justify-between gap-1 mb-0.5">
          <div class="flex items-center gap-1.5 min-w-0">
            <span
              class="text-xs font-bold text-slate-900 truncate group-hover:text-blue-600"
            >
              {{ item.symbol || shortAddress(item.address) }}
            </span>
            <span class="text-[10px] text-slate-400 shrink-0">{{
              chainLabel(item.chain)
            }}</span>
          </div>
          <span
            class="text-[9px] font-bold px-1.5 py-0.5 rounded uppercase shrink-0"
            :class="
              item.status === 'active'
                ? 'bg-[#0f1d33] text-white'
                : item.status === 'watch'
                ? 'bg-blue-50 text-blue-700 border border-blue-200'
                : 'bg-slate-100 text-slate-500'
            "
          >
            {{ item.status }}
          </span>
        </div>
        <div
          v-if="item.note"
          class="text-[10.5px] text-slate-500 truncate mt-0.5"
        >
          {{ item.note }}
        </div>
        <div class="flex items-center justify-between mt-1">
          <span class="text-[10px] font-mono text-slate-400">
            {{ shortAddress(item.address) }}
          </span>
          <span class="text-[10px] font-mono text-slate-400">
            {{ formatTime(item.updated_at) }}
          </span>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
  import { reactive, ref } from 'vue';
  import { useAppStore, useRadarStore } from '@/store';
  import { RADAR_SOURCES, WATCHLIST_CHAIN_OPTIONS } from '@/constants/radar';
  import { chainLabel, formatTime, shortAddress } from '@/utils/format';

  defineEmits<{
    (e: 'selectSource', source: string): void;
    (e: 'selectToken', chain: string, address: string): void;
  }>();

  const appStore = useAppStore();
  const radarStore = useRadarStore();

  const addFormOpen = ref(false);
  const submitting = ref(false);
  const form = reactive({
    chain: 'solana',
    address: '',
    note: '',
  });

  const latestStatus = (scanner: string) =>
    radarStore.latestJobByScanner[scanner]?.status || 'idle';

  const statusDotClass = (scanner: string) => {
    const st = latestStatus(scanner);
    if (st === 'ok') return 'bg-emerald-500';
    if (st === 'warning') return 'bg-amber-500';
    if (st === 'error') return 'bg-rose-500';
    return 'bg-slate-300';
  };

  const submitAddWatch = async () => {
    const addr = form.address.trim();
    if (!addr) return;
    submitting.value = true;
    try {
      await radarStore.saveWatchItem({
        chain: form.chain,
        address: addr,
        status: 'active',
        note: form.note.trim() || undefined,
      });
      form.address = '';
      form.note = '';
      addFormOpen.value = false;
      appStore.showToast('已加入观察名单', 'success');
    } catch {
      appStore.showToast('添加观察名单失败', 'error');
    } finally {
      submitting.value = false;
    }
  };
</script>
