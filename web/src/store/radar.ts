import { defineStore } from 'pinia';
import {
  queryJobs,
  queryPushes,
  querySignals,
  queryToken,
  queryWatchlist,
  upsertWatchlist,
  type FilterOptions,
  type ScannerRun,
  type SignalEvent,
  type TokenDetailResponse,
  type WatchlistItem,
  type WatchlistUpsertPayload,
} from '@/api/radar';

export type FeedMode = 'signals' | 'pushes';

export interface SignalFiltersState {
  source: string;
  chain: string;
  symbol: string;
  signal_type: string;
  priority: string;
  time_range: string;
  watchlist_only: string;
  page: number;
  pageSize: number;
}

export const useRadarStore = defineStore('radar', {
  state: () => ({
    feedMode: 'signals' as FeedMode,
    loadingFeed: false,
    signals: [] as SignalEvent[],
    total: 0,
    filters: {
      source: '',
      chain: '',
      symbol: '',
      signal_type: '',
      priority: '',
      time_range: '24h',
      watchlist_only: '',
      page: 1,
      pageSize: 20,
    } as SignalFiltersState,
    filterOptions: {
      sources: [],
      chains: [],
      signalTypes: [],
      priorities: [],
      timeRanges: ['1h', '6h', '24h', '7d'],
    } as FilterOptions,
    // 5 张顶部统计卡数字（均通过 pageSize=1 接口取 total）
    stats: {
      total24h: 0,
      highPriority: 0,
      resonance: 0,
      pushedTg: 0,
      watchlistHit: 0,
    },
    // 各来源计数（用于左栏雷达来源计数）
    sourceCounts: {} as Record<string, number>,
    jobs: [] as ScannerRun[],
    loadingJobs: false,
    watchlist: [] as WatchlistItem[],
    loadingWatchlist: false,
    selectedTokenKey: '', // 格式: chain:address
    tokenDetail: null as TokenDetailResponse | null,
    loadingToken: false,
    snapshotSource: 'all',
  }),
  getters: {
    latestJobByScanner(state): Record<string, ScannerRun> {
      const map: Record<string, ScannerRun> = {};
      state.jobs.forEach((job) => {
        if (job.scanner && !map[job.scanner]) {
          map[job.scanner] = job;
        }
      });
      return map;
    },
  },
  actions: {
    buildSignalParams(
      overrides?: Partial<SignalFiltersState>
    ): Record<string, unknown> {
      const merged = { ...this.filters, ...overrides };
      const params: Record<string, unknown> = {
        time_range: merged.time_range || '24h',
        page: merged.page,
        pageSize: merged.pageSize,
      };
      if (merged.source) params.source = merged.source;
      if (merged.chain) params.chain = merged.chain;
      if (merged.symbol) params.symbol = merged.symbol;
      if (merged.signal_type) params.signal_type = merged.signal_type;
      if (merged.priority) params.priority = merged.priority;
      if (merged.watchlist_only) params.watchlist_only = merged.watchlist_only;
      return params;
    },
    async fetchFeed() {
      this.loadingFeed = true;
      try {
        const params = this.buildSignalParams();
        const fetcher = this.feedMode === 'pushes' ? queryPushes : querySignals;
        const { data } = await fetcher(params);
        this.signals = data.items || [];
        this.total = data.total ?? this.signals.length;
        if (data.filters) {
          this.filterOptions = {
            sources: data.filters.sources || [],
            chains: data.filters.chains || [],
            signalTypes: data.filters.signalTypes || [],
            priorities: data.filters.priorities || [],
            timeRanges: data.filters.timeRanges || ['1h', '6h', '24h', '7d'],
          };
        }
        // 默认选中第一条信号对应的标的
        if (!this.selectedTokenKey && this.signals.length > 0) {
          const first = this.signals[0];
          if (first.chain && first.address) {
            this.selectToken(first.chain, first.address);
          }
        }
      } finally {
        this.loadingFeed = false;
      }
    },
    async fetchStats() {
      try {
        const range = this.filters.time_range || '24h';
        const [allRes, highRes, resRes, pushRes, watchRes] = await Promise.all([
          querySignals({ time_range: range, page: 1, pageSize: 1 }),
          querySignals({
            time_range: range,
            priority: 'high',
            page: 1,
            pageSize: 1,
          }),
          querySignals({
            time_range: range,
            signal_type: 'resonance',
            page: 1,
            pageSize: 1,
          }),
          queryPushes({ time_range: range, page: 1, pageSize: 1 }),
          querySignals({
            time_range: range,
            watchlist_only: 'true',
            page: 1,
            pageSize: 1,
          }),
        ]);
        this.stats = {
          total24h: allRes.data.total ?? 0,
          highPriority: highRes.data.total ?? 0,
          resonance: resRes.data.total ?? 0,
          pushedTg: pushRes.data.total ?? 0,
          watchlistHit: watchRes.data.total ?? 0,
        };

        // 并发获取 S1/S2/S3/S5/S7 各自计数
        const srcKeys = ['s1', 's2', 's3', 's5', 's7'];
        const srcResults = await Promise.all(
          srcKeys.map((src) =>
            querySignals({
              time_range: range,
              source: src,
              page: 1,
              pageSize: 1,
            })
          )
        );
        const nextCounts: Record<string, number> = {};
        srcKeys.forEach((src, idx) => {
          nextCounts[src] = srcResults[idx].data.total ?? 0;
        });
        this.sourceCounts = nextCounts;
      } catch (err) {
        console.warn('[radar] fetch stats failed', err);
      }
    },
    async fetchJobs(limit = 30) {
      this.loadingJobs = true;
      try {
        const { data } = await queryJobs({ limit });
        this.jobs = data.items || [];
      } finally {
        this.loadingJobs = false;
      }
    },
    async fetchWatchlist() {
      this.loadingWatchlist = true;
      try {
        const { data } = await queryWatchlist();
        this.watchlist = data.items || [];
      } finally {
        this.loadingWatchlist = false;
      }
    },
    async saveWatchItem(payload: WatchlistUpsertPayload) {
      await upsertWatchlist(payload);
      await Promise.all([this.fetchWatchlist(), this.fetchStats()]);
      if (this.selectedTokenKey === `${payload.chain}:${payload.address}`) {
        await this.fetchTokenDetail(payload.chain, payload.address);
      }
    },
    async pauseWatchItem(item: WatchlistItem) {
      // 后端无删除接口，按约束调用 POST /watchlist 将 status 置为 paused
      await this.saveWatchItem({
        chain: item.chain,
        address: item.address,
        symbol: item.symbol,
        name: item.name,
        status: 'paused',
        note: item.note,
      });
    },
    async selectToken(chain: string, address: string) {
      if (!chain || !address) return;
      this.selectedTokenKey = `${chain}:${address}`;
      this.snapshotSource = 'all';
      await this.fetchTokenDetail(chain, address);
    },
    async fetchTokenDetail(chain: string, address: string) {
      this.loadingToken = true;
      try {
        const { data } = await queryToken(chain, address);
        this.tokenDetail = data;
      } catch (err) {
        console.warn('[radar] fetch token detail failed', err);
        this.tokenDetail = null;
      } finally {
        this.loadingToken = false;
      }
    },
    async exportCurrentCsv(): Promise<SignalEvent[]> {
      const fetcher = this.feedMode === 'pushes' ? queryPushes : querySignals;
      const params = this.buildSignalParams({ page: 1, pageSize: 200 });
      const { data } = await fetcher(params);
      return data.items || [];
    },
  },
});
