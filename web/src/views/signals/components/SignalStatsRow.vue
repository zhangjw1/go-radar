<template>
  <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2.5 shrink-0">
    <button
      type="button"
      class="bg-white border rounded-lg p-3 text-left transition-all cursor-pointer"
      :class="
        isAllActive
          ? 'border-blue-500 ring-1 ring-blue-500/20'
          : 'border-slate-200 hover:border-slate-300'
      "
      @click="$emit('quickFilter', 'all')"
    >
      <div class="text-[11px] text-slate-500 font-medium">24h 信号</div>
      <div class="text-xl font-mono font-bold text-slate-900 mt-1">
        {{ radarStore.stats.total24h }}
      </div>
    </button>

    <button
      type="button"
      class="bg-white border rounded-lg p-3 text-left transition-all cursor-pointer"
      :class="
        radarStore.filters.priority === 'high'
          ? 'border-rose-500 ring-1 ring-rose-500/20 bg-rose-50/20'
          : 'border-slate-200 hover:border-slate-300'
      "
      @click="$emit('quickFilter', 'high')"
    >
      <div class="text-[11px] text-slate-500 font-medium">高优先级</div>
      <div class="text-xl font-mono font-bold text-rose-600 mt-1">
        {{ radarStore.stats.highPriority }}
      </div>
    </button>

    <button
      type="button"
      class="bg-white border rounded-lg p-3 text-left transition-all cursor-pointer"
      :class="
        radarStore.filters.signal_type === 'resonance'
          ? 'border-amber-500 ring-1 ring-amber-500/20 bg-amber-50/20'
          : 'border-slate-200 hover:border-slate-300'
      "
      @click="$emit('quickFilter', 'resonance')"
    >
      <div class="text-[11px] text-slate-500 font-medium">⚡ 跨源共振</div>
      <div class="text-xl font-mono font-bold text-amber-600 mt-1">
        {{ radarStore.stats.resonance }}
      </div>
    </button>

    <button
      type="button"
      class="bg-white border rounded-lg p-3 text-left transition-all cursor-pointer"
      :class="
        radarStore.feedMode === 'pushes'
          ? 'border-blue-500 ring-1 ring-blue-500/20 bg-blue-50/20'
          : 'border-slate-200 hover:border-slate-300'
      "
      @click="$emit('quickFilter', 'pushes')"
    >
      <div class="text-[11px] text-slate-500 font-medium">已推送 TG</div>
      <div class="text-xl font-mono font-bold text-blue-600 mt-1">
        {{ radarStore.stats.pushedTg }}
      </div>
    </button>

    <button
      type="button"
      class="bg-white border rounded-lg p-3 text-left transition-all cursor-pointer col-span-2 sm:col-span-1"
      :class="
        radarStore.filters.watchlist_only === 'true'
          ? 'border-emerald-500 ring-1 ring-emerald-500/20 bg-emerald-50/20'
          : 'border-slate-200 hover:border-slate-300'
      "
      @click="$emit('quickFilter', 'watchlist')"
    >
      <div class="text-[11px] text-slate-500 font-medium">★ 命中观察</div>
      <div class="text-xl font-mono font-bold text-emerald-600 mt-1">
        {{ radarStore.stats.watchlistHit }}
      </div>
    </button>
  </div>
</template>

<script setup lang="ts">
  import { computed } from 'vue';
  import { useRadarStore } from '@/store';

  defineEmits<{
    (
      e: 'quickFilter',
      type: 'all' | 'high' | 'resonance' | 'pushes' | 'watchlist'
    ): void;
  }>();

  const radarStore = useRadarStore();

  const isAllActive = computed(
    () =>
      radarStore.feedMode === 'signals' &&
      !radarStore.filters.priority &&
      !radarStore.filters.signal_type &&
      !radarStore.filters.watchlist_only
  );
</script>
