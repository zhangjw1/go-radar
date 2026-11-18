<template>
  <header
    class="h-14 bg-white border-b border-slate-200 px-4 md:px-6 flex items-center justify-between shrink-0 z-30 select-none"
  >
    <!-- 左侧：Logo 与模式切换 -->
    <div class="flex items-center gap-3 min-w-0">
      <button
        type="button"
        class="flex items-center gap-2.5 cursor-pointer shrink-0"
        @click="goMode('signals')"
      >
        <div
          class="w-8 h-8 rounded-lg bg-[#09182b] flex items-center justify-center shadow-xs border border-slate-800 shrink-0"
        >
          <svg
            class="w-4 h-4 text-cyan-400"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="12" cy="12" r="2" />
            <path d="M12 12 19 5" />
            <path d="M4.9 19.1A10 10 0 1 1 19.1 19.1" />
            <path d="M8.5 15.5a5 5 0 1 1 7 0" />
          </svg>
        </div>
        <span class="text-lg font-bold text-slate-900 tracking-tight"
          >Go-Radar</span
        >
      </button>

      <span class="text-slate-300 font-light mx-0.5 hidden lg:inline">|</span>

      <!-- 模式切换：信号雷达 / 钱包监控 -->
      <div
        class="ml-1 bg-slate-100 p-0.5 rounded-lg border border-slate-200/80 flex items-center text-xs font-semibold shrink-0"
      >
        <button
          type="button"
          class="px-3 py-1 rounded-md transition-all cursor-pointer"
          :class="
            !isWalletMode
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-500 hover:text-slate-800'
          "
          @click="goMode('signals')"
        >
          信号雷达
          <span
            class="ml-1 font-mono text-[10px]"
            :class="!isWalletMode ? 'text-blue-600' : 'text-slate-400'"
            >{{ radarStore.stats.total24h }}</span
          >
        </button>
        <button
          type="button"
          class="px-3 py-1 rounded-md transition-all cursor-pointer"
          :class="
            isWalletMode
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-500 hover:text-slate-800'
          "
          @click="goMode('wallets')"
        >
          钱包监控
          <span
            class="ml-1 font-mono text-[10px]"
            :class="isWalletMode ? 'text-blue-600' : 'text-slate-400'"
            >{{ insiderStore.wallets.length }}</span
          >
        </button>
      </div>
    </div>

    <!-- 右侧：Scheduler 只读状态 + 自动刷新 + 高优/推送计数 + 系统设置 -->
    <div class="flex items-center gap-3 sm:gap-4 shrink-0">
      <!-- Scheduler 状态（只读） -->
      <div
        class="hidden sm:flex items-center gap-1.5 text-xs text-slate-600 font-medium px-2 py-1 rounded-md bg-slate-50 border border-slate-200/80"
        title="后端调度器状态只读（由服务端配置决定）"
      >
        <span
          class="w-2 h-2 rounded-full"
          :class="
            appStore.schedulerEnabled
              ? 'bg-emerald-500 animate-pulse'
              : 'bg-slate-400'
          "
        />
        <span>{{
          appStore.schedulerEnabled ? 'Scheduler 运行中' : 'Scheduler 未开启'
        }}</span>
      </div>

      <!-- 前端 30 秒自动刷新开关 -->
      <button
        type="button"
        class="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-md border transition-colors cursor-pointer"
        :class="
          appStore.autoRefresh
            ? 'bg-emerald-50/70 border-emerald-200 text-emerald-700'
            : 'bg-white border-slate-200 text-slate-500 hover:bg-slate-50'
        "
        title="开启/暂停前端每 30 秒自动轮询"
        @click="appStore.toggleAutoRefresh()"
      >
        <span
          class="w-1.5 h-1.5 rounded-full"
          :class="appStore.autoRefresh ? 'bg-emerald-500' : 'bg-slate-400'"
        />
        <span class="hidden md:inline">{{
          appStore.autoRefresh ? '自动刷新 30s' : '刷新已暂停'
        }}</span>
      </button>

      <!-- 高优与已推送计数 -->
      <div
        class="text-[11px] leading-tight text-slate-500 font-mono text-right hidden md:block"
      >
        <div>
          高优信号 :
          <span class="font-semibold text-rose-600">{{
            radarStore.stats.highPriority
          }}</span>
        </div>
        <div>
          已推 TG :
          <span class="font-semibold text-slate-700">{{
            radarStore.stats.pushedTg
          }}</span>
        </div>
      </div>

      <!-- 系统设置按钮 -->
      <button
        type="button"
        class="flex items-center gap-1.5 bg-[#09182b] hover:bg-slate-800 text-white text-xs font-semibold px-3 py-1.5 rounded-md shadow-xs transition-colors cursor-pointer"
        @click="openSystemSettings()"
      >
        <svg
          class="w-3.5 h-3.5 text-cyan-400"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="12" cy="12" r="3" />
          <path
            d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
          />
        </svg>
        <span class="hidden sm:inline">系统设置</span>
      </button>
    </div>
  </header>

  <!-- 移动端三栏页签切换 (<768px) -->
  <div
    class="md:hidden flex border-b border-slate-200 bg-white text-xs font-semibold shrink-0"
  >
    <button
      type="button"
      class="flex-1 py-2 text-center border-b-2 cursor-pointer"
      :class="
        appStore.mobilePane === 'left'
          ? 'border-blue-600 text-blue-600'
          : 'border-transparent text-slate-600'
      "
      @click="appStore.applyMobilePane('left')"
    >
      {{ isWalletMode ? '同步 & 钱包列表' : '雷达源 & 观察名单' }}
    </button>
    <button
      type="button"
      class="flex-1 py-2 text-center border-b-2 cursor-pointer"
      :class="
        appStore.mobilePane === 'center'
          ? 'border-blue-600 text-blue-600'
          : 'border-transparent text-slate-600'
      "
      @click="appStore.applyMobilePane('center')"
    >
      {{ isWalletMode ? '持仓 & 交易' : '信号流' }}
    </button>
    <button
      type="button"
      class="flex-1 py-2 text-center border-b-2 cursor-pointer"
      :class="
        appStore.mobilePane === 'right'
          ? 'border-blue-600 text-blue-600'
          : 'border-transparent text-slate-600'
      "
      @click="appStore.applyMobilePane('right')"
    >
      {{ isWalletMode ? '盈亏 & 告警' : '标的详情 & 快照' }}
    </button>
  </div>
</template>

<script setup lang="ts">
  import { computed } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { useAppStore, useInsiderStore, useRadarStore } from '@/store';

  const route = useRoute();
  const router = useRouter();
  const appStore = useAppStore();
  const radarStore = useRadarStore();
  const insiderStore = useInsiderStore();

  const isWalletMode = computed(() => route.path.startsWith('/wallets'));

  const goMode = (mode: 'signals' | 'wallets') => {
    appStore.applyMobilePane('center');
    if (mode === 'signals') {
      router.push('/signals');
    } else {
      router.push('/wallets');
    }
  };

  const openSystemSettings = () => {
    if (route.path.startsWith('/signals')) {
      router.replace({
        path: '/signals',
        query: { ...route.query, settings: appStore.settingsTab || 'scanner' },
      });
    } else {
      appStore.openSettings(appStore.settingsTab || 'scanner');
    }
  };
</script>
