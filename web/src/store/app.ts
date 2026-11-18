import { defineStore } from 'pinia';
import { queryHealth, type HealthJobSpec } from '@/api/radar';
import type { SettingTabKey } from '@/constants/radar';

export type MobilePane = 'left' | 'center' | 'right';

export const useAppStore = defineStore('app', {
  state: () => ({
    schedulerEnabled: false,
    schedulerJobs: [] as HealthJobSpec[],
    databaseOk: true,
    autoRefresh: true,
    mobilePane: 'center' as MobilePane,
    toastVisible: false,
    toastMessage: '',
    toastType: 'info' as 'info' | 'success' | 'error',
    toastTimer: null as ReturnType<typeof setTimeout> | null,
    rawModalOpen: false,
    rawModalTitle: '',
    rawModalBadge: '',
    rawModalJson: '',
    settingsOpen: false,
    settingsTab: 'scanner' as SettingTabKey,
  }),
  actions: {
    async fetchHealth() {
      try {
        const { data } = await queryHealth();
        this.schedulerEnabled = Boolean(data.scheduler_enabled);
        this.databaseOk = Boolean(data.database_ok);
        this.schedulerJobs = data.jobs || [];
      } catch (err) {
        console.warn('[app] fetch health failed', err);
      }
    },
    toggleAutoRefresh() {
      this.autoRefresh = !this.autoRefresh;
      this.showToast(
        this.autoRefresh ? '已开启 30 秒自动轮询' : '已暂停前端自动轮询',
        'info'
      );
    },
    applyMobilePane(pane: MobilePane) {
      this.mobilePane = pane;
    },
    showToast(message: string, type: 'info' | 'success' | 'error' = 'info') {
      if (this.toastTimer) {
        clearTimeout(this.toastTimer);
      }
      this.toastMessage = message;
      this.toastType = type;
      this.toastVisible = true;
      this.toastTimer = setTimeout(() => {
        this.toastVisible = false;
      }, 2600);
    },
    openRawModal(title: string, badge: string, rawJson?: string) {
      this.rawModalTitle = title;
      this.rawModalBadge = badge;
      let formatted = rawJson || '{}';
      try {
        formatted = JSON.stringify(JSON.parse(formatted), null, 2);
      } catch {
        // 保持原始文本
      }
      this.rawModalJson = formatted;
      this.rawModalOpen = true;
    },
    closeRawModal() {
      this.rawModalOpen = false;
    },
    openSettings(tab: SettingTabKey = 'scanner') {
      this.settingsTab = tab;
      this.settingsOpen = true;
    },
    closeSettings() {
      this.settingsOpen = false;
    },
  },
});
