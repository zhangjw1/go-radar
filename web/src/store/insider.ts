import { defineStore } from 'pinia';
import {
  createInsiderWallet,
  deleteInsiderWallet,
  queryAlertHistory,
  queryAlertRules,
  queryInsiderAnalytics,
  queryInsiderPortfolio,
  queryInsiderSyncStatus,
  queryInsiderTransactions,
  queryInsiderWallets,
  queryNotificationChannels,
  saveAlertRule,
  saveNotificationChannel,
  triggerInsiderSync,
  updateInsiderEngine,
  updateInsiderWallet,
  type AlertHistory,
  type AlertRule,
  type InsiderTransaction,
  type InsiderWallet,
  type NotificationChannel,
  type SyncStatus,
  type TokenHolding,
  type WalletAnalytics,
} from '@/api/insider';

export interface WalletOverviewItem extends InsiderWallet {
  balanceUsd: number;
  winRate: number;
  totalPnl: number;
}

export const useInsiderStore = defineStore('insider', {
  state: () => ({
    loadingWallets: false,
    wallets: [] as WalletOverviewItem[],
    selectedWalletId: null as number | null,
    loadingDetail: false,
    holdings: [] as TokenHolding[],
    transactions: [] as InsiderTransaction[],
    analytics: null as WalletAnalytics | null,
    period: '7d' as '7d' | '30d' | 'all',
    syncStatus: {
      syncing: false,
      last_error: '',
      engine: 'service',
    } as SyncStatus,
    rules: [] as AlertRule[],
    channels: [] as NotificationChannel[],
    history: [] as AlertHistory[],
    editModalOpen: false,
    editingWallet: null as InsiderWallet | null,
    syncPollTimer: null as ReturnType<typeof setInterval> | null,
  }),
  getters: {
    selectedWallet(state): WalletOverviewItem | null {
      return (
        state.wallets.find((item) => item.id === state.selectedWalletId) || null
      );
    },
  },
  actions: {
    async fetchWallets() {
      this.loadingWallets = true;
      try {
        const { data } = await queryInsiderWallets();
        const rawList = data.items || [];
        // 并发请求每个钱包的 portfolio 与 7d analytics
        const enriched = await Promise.all(
          rawList.map(async (wallet) => {
            try {
              const [portRes, anaRes] = await Promise.all([
                queryInsiderPortfolio(wallet.id),
                queryInsiderAnalytics(wallet.id, '7d'),
              ]);
              const holdings = portRes.data.items || [];
              const balanceUsd = holdings.reduce(
                (sum, h) => sum + (h.current_value || h.usd_value || 0),
                0
              );
              const ana = anaRes.data.item;
              return {
                ...wallet,
                balanceUsd,
                winRate: ana?.win_rate ?? 0,
                totalPnl: ana?.total_pnl ?? 0,
              };
            } catch {
              return {
                ...wallet,
                balanceUsd: 0,
                winRate: 0,
                totalPnl: 0,
              };
            }
          })
        );
        this.wallets = enriched;
        if (
          this.wallets.length > 0 &&
          (!this.selectedWalletId ||
            !this.wallets.some((w) => w.id === this.selectedWalletId))
        ) {
          await this.selectWallet(this.wallets[0].id);
        }
      } finally {
        this.loadingWallets = false;
      }
    },
    async selectWallet(id: number) {
      this.selectedWalletId = id;
      await this.fetchWalletDetail(id);
    },
    async fetchWalletDetail(id: number) {
      this.loadingDetail = true;
      try {
        const [portRes, txRes, anaRes] = await Promise.all([
          queryInsiderPortfolio(id),
          queryInsiderTransactions(id, 100),
          queryInsiderAnalytics(id, this.period),
        ]);
        this.holdings = portRes.data.items || [];
        this.transactions = txRes.data.items || [];
        this.analytics = anaRes.data.item || null;
      } finally {
        this.loadingDetail = false;
      }
    },
    async changePeriod(period: '7d' | '30d' | 'all') {
      this.period = period;
      if (this.selectedWalletId) {
        await this.fetchWalletDetail(this.selectedWalletId);
      }
    },
    async addWallet(address: string, label: string) {
      const { data } = await createInsiderWallet({ address, label });
      await this.fetchWallets();
      if (data.item?.id) {
        await this.selectWallet(data.item.id);
      }
    },
    async renameWallet(id: number, label: string) {
      await updateInsiderWallet(id, label);
      await this.fetchWallets();
    },
    async removeWallet(id: number) {
      await deleteInsiderWallet(id);
      if (this.selectedWalletId === id) {
        this.selectedWalletId = null;
        this.holdings = [];
        this.transactions = [];
        this.analytics = null;
      }
      await this.fetchWallets();
    },
    async fetchSyncStatus() {
      try {
        const { data } = await queryInsiderSyncStatus();
        if (data.item) {
          this.syncStatus = data.item;
        }
      } catch (err) {
        console.warn('[insider] fetch sync status failed', err);
      }
    },
    async triggerSync(onDone?: (err?: string) => void) {
      await triggerInsiderSync();
      await this.fetchSyncStatus();
      if (this.syncPollTimer) clearInterval(this.syncPollTimer);
      let attempts = 0;
      this.syncPollTimer = setInterval(async () => {
        attempts += 1;
        await this.fetchSyncStatus();
        if (!this.syncStatus.syncing || attempts >= 20) {
          if (this.syncPollTimer) {
            clearInterval(this.syncPollTimer);
            this.syncPollTimer = null;
          }
          await this.fetchWallets();
          if (this.selectedWalletId) {
            await this.fetchWalletDetail(this.selectedWalletId);
          }
          await this.fetchAlertData();
          onDone?.(this.syncStatus.last_error || undefined);
        }
      }, 2000);
    },
    async fetchAlertData() {
      try {
        const [rulesRes, channelsRes, historyRes] = await Promise.all([
          queryAlertRules(),
          queryNotificationChannels(),
          queryAlertHistory(),
        ]);
        this.rules = rulesRes.data.items || [];
        this.channels = channelsRes.data.items || [];
        this.history = historyRes.data.items || [];
      } catch (err) {
        console.warn('[insider] fetch alert data failed', err);
      }
    },
    async changeEngine(engine: 'service' | 'legacy') {
      await updateInsiderEngine(engine);
      await this.fetchSyncStatus();
    },
    async saveRule(rule: AlertRule) {
      await saveAlertRule(rule);
      await this.fetchAlertData();
    },
    async saveChannel(channel: NotificationChannel) {
      await saveNotificationChannel(channel);
      await this.fetchAlertData();
    },
    openEditModal(wallet: InsiderWallet) {
      this.editingWallet = wallet;
      this.editModalOpen = true;
    },
    closeEditModal() {
      this.editModalOpen = false;
      this.editingWallet = null;
    },
  },
});
