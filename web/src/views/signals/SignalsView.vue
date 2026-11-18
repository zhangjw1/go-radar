<template>
  <div class="flex-1 flex overflow-hidden min-h-0 w-full">
    <!-- 左栏：雷达来源 + 观察名单 -->
    <RadarSidebar
      :class="appStore.mobilePane === 'left' ? 'flex' : 'hidden md:flex'"
      @select-source="onSelectSource"
      @select-token="onSelectToken"
    />

    <!-- 中栏：浅灰底 + 5张统计卡 + 信号表卡片 -->
    <section
      class="flex-1 h-full min-w-0 flex-col bg-[#f3f5f8] p-3 md:p-4 gap-3 overflow-hidden"
      :class="appStore.mobilePane === 'center' ? 'flex' : 'hidden md:flex'"
    >
      <SignalStatsRow @quick-filter="onQuickFilter" />
      <SignalFeedTable
        @change-feed-mode="onChangeFeedMode"
        @update-filter="onUpdateFilter"
        @clear-source="onSelectSource('')"
        @toggle-watchlist-only="onToggleWatchlistOnly"
        @change-page="onChangePage"
        @change-page-size="onChangePageSize"
        @select-token="onSelectToken"
      />
    </section>

    <!-- 右栏：标的详情与历史快照 -->
    <TokenDetailPane
      :class="appStore.mobilePane === 'right' ? 'flex' : 'hidden md:flex'"
    />
  </div>
</template>

<script setup lang="ts">
  import { onMounted, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { useAppStore, useRadarStore, type FeedMode } from '@/store';
  import RadarSidebar from './components/RadarSidebar.vue';
  import SignalStatsRow from './components/SignalStatsRow.vue';
  import SignalFeedTable from './components/SignalFeedTable.vue';
  import TokenDetailPane from './components/TokenDetailPane.vue';

  const route = useRoute();
  const router = useRouter();
  const appStore = useAppStore();
  const radarStore = useRadarStore();

  const syncQuery = (extra?: Record<string, string | undefined>) => {
    const query: Record<string, string> = {};
    if (radarStore.filters.source) query.source = radarStore.filters.source;
    if (radarStore.feedMode === 'pushes') query.feed = 'pushes';
    if (radarStore.selectedTokenKey) query.token = radarStore.selectedTokenKey;
    if (typeof route.query.settings === 'string' && route.query.settings) {
      query.settings = route.query.settings;
    }
    if (extra) {
      Object.entries(extra).forEach(([k, v]) => {
        if (v) query[k] = v;
        else delete query[k];
      });
    }
    router.replace({ path: '/signals', query });
  };

  const applyRouteQuery = async () => {
    const { source, feed, token } = route.query;
    radarStore.filters.source = typeof source === 'string' ? source : '';
    radarStore.feedMode = feed === 'pushes' ? 'pushes' : 'signals';
    await Promise.all([
      radarStore.fetchFeed(),
      radarStore.fetchStats(),
      radarStore.fetchWatchlist(),
    ]);
    if (typeof token === 'string' && token.includes(':')) {
      const idx = token.indexOf(':');
      const chain = token.slice(0, idx);
      const address = token.slice(idx + 1);
      if (chain && address) {
        await radarStore.selectToken(chain, address);
      }
    }
  };

  const onSelectSource = async (source: string) => {
    radarStore.filters.source =
      radarStore.filters.source === source ? '' : source;
    radarStore.filters.page = 1;
    syncQuery();
    await radarStore.fetchFeed();
  };

  const onChangeFeedMode = async (mode: FeedMode) => {
    radarStore.feedMode = mode;
    radarStore.filters.page = 1;
    syncQuery();
    await radarStore.fetchFeed();
  };

  const onQuickFilter = async (
    type: 'all' | 'high' | 'resonance' | 'pushes' | 'watchlist'
  ) => {
    if (type === 'all') {
      radarStore.feedMode = 'signals';
      radarStore.filters.priority = '';
      radarStore.filters.signal_type = '';
      radarStore.filters.watchlist_only = '';
    } else if (type === 'high') {
      radarStore.feedMode = 'signals';
      radarStore.filters.priority =
        radarStore.filters.priority === 'high' ? '' : 'high';
    } else if (type === 'resonance') {
      radarStore.feedMode = 'signals';
      radarStore.filters.signal_type =
        radarStore.filters.signal_type === 'resonance' ? '' : 'resonance';
    } else if (type === 'pushes') {
      radarStore.feedMode =
        radarStore.feedMode === 'pushes' ? 'signals' : 'pushes';
    } else if (type === 'watchlist') {
      radarStore.filters.watchlist_only =
        radarStore.filters.watchlist_only === 'true' ? '' : 'true';
    }
    radarStore.filters.page = 1;
    syncQuery();
    await radarStore.fetchFeed();
  };

  const onUpdateFilter = async (key: string, value: string) => {
    (radarStore.filters as Record<string, unknown>)[key] = value;
    radarStore.filters.page = 1;
    await radarStore.fetchFeed();
    if (key === 'time_range') {
      await radarStore.fetchStats();
    }
  };

  const onToggleWatchlistOnly = async () => {
    radarStore.filters.watchlist_only =
      radarStore.filters.watchlist_only === 'true' ? '' : 'true';
    radarStore.filters.page = 1;
    await radarStore.fetchFeed();
  };

  const onChangePage = async (page: number) => {
    radarStore.filters.page = page;
    await radarStore.fetchFeed();
  };

  const onChangePageSize = async (size: number) => {
    radarStore.filters.pageSize = size;
    radarStore.filters.page = 1;
    await radarStore.fetchFeed();
  };

  const onSelectToken = async (chain: string, address: string) => {
    await radarStore.selectToken(chain, address);
    syncQuery({ token: `${chain}:${address}` });
    if (window.innerWidth < 768) {
      appStore.applyMobilePane('right');
    }
  };

  watch(
    () => [route.query.source, route.query.feed],
    () => {
      if (route.path === '/signals') {
        const src =
          typeof route.query.source === 'string' ? route.query.source : '';
        const fd: FeedMode =
          route.query.feed === 'pushes' ? 'pushes' : 'signals';
        if (src !== radarStore.filters.source || fd !== radarStore.feedMode) {
          radarStore.filters.source = src;
          radarStore.feedMode = fd;
          radarStore.fetchFeed().catch(() => undefined);
        }
      }
    }
  );

  onMounted(() => {
    applyRouteQuery().catch(() => undefined);
  });
</script>
