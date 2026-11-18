<template>
  <footer
    class="h-8 bg-white border-t border-slate-200 px-4 md:px-6 flex items-center justify-between text-[11px] text-slate-500 shrink-0 font-mono overflow-x-auto"
  >
    <div class="flex items-center gap-3 whitespace-nowrap">
      <span
        class="inline-flex items-center gap-1.5 font-sans font-semibold"
        :class="appStore.databaseOk ? 'text-emerald-600' : 'text-rose-600'"
      >
        <span
          class="w-1.5 h-1.5 rounded-full"
          :class="appStore.databaseOk ? 'bg-emerald-500' : 'bg-rose-500'"
        />
        Go-Radar
      </span>
      <span class="text-slate-300">|</span>

      <button
        v-for="item in scannerStatuses"
        :key="item.key"
        type="button"
        class="inline-flex items-center gap-1 hover:text-slate-900 transition-colors cursor-pointer"
        :title="`查看 ${item.label} 最近运行任务`"
        @click="openJobsTab()"
      >
        <span class="w-1.5 h-1.5 rounded-full" :class="dotClass(item.status)" />
        <span>{{ item.shortLabel }}:</span>
        <strong class="text-slate-700 font-medium">{{ item.lastRun }}</strong>
      </button>
    </div>

    <div class="hidden sm:flex items-center gap-3 shrink-0 font-sans">
      <button
        type="button"
        class="text-blue-600 hover:underline cursor-pointer"
        @click="openJobsTab()"
      >
        任务记录 →
      </button>
    </div>
  </footer>
</template>

<script setup lang="ts">
  import { computed } from 'vue';
  import { useAppStore, useRadarStore } from '@/store';
  import { RADAR_SOURCES } from '@/constants/radar';
  import { formatRelTime } from '@/utils/format';

  const appStore = useAppStore();
  const radarStore = useRadarStore();

  const scannerStatuses = computed(() => {
    const latest = radarStore.latestJobByScanner;
    const base = RADAR_SOURCES.map((s) => {
      const job = latest[s.key];
      return {
        key: s.key,
        label: s.label,
        shortLabel: s.shortLabel,
        status: job?.status || 'idle',
        lastRun: job ? formatRelTime(job.started_at) : '待运行',
      };
    });
    const insiderJob = latest.insider;
    base.push({
      key: 'insider',
      label: 'Insider 钱包',
      shortLabel: 'INSIDER',
      status: insiderJob?.status || 'idle',
      lastRun: insiderJob ? formatRelTime(insiderJob.started_at) : '待运行',
    });
    return base;
  });

  const dotClass = (status: string) => {
    if (status === 'ok') return 'bg-emerald-500';
    if (status === 'warning') return 'bg-amber-500';
    if (status === 'error') return 'bg-rose-500';
    return 'bg-slate-300';
  };

  const openJobsTab = () => {
    appStore.openSettings('jobs');
  };
</script>
