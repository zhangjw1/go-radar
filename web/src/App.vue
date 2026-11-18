<template>
  <div
    class="flex flex-col h-screen w-screen overflow-hidden bg-[#f3f5f8] text-slate-800 antialiased"
  >
    <AppHeader />
    <main class="flex-1 flex overflow-hidden min-h-0">
      <router-view />
    </main>
    <AppFooter />
    <RawJsonModal />
    <SettingsModal />
    <ToastContainer />
  </div>
</template>

<script setup lang="ts">
  import { onMounted, onUnmounted, watch } from 'vue';
  import { useRoute } from 'vue-router';
  import { useAppStore, useInsiderStore, useRadarStore } from '@/store';
  import type { SettingTabKey } from '@/constants/radar';
  import AppHeader from '@/components/AppHeader.vue';
  import AppFooter from '@/components/AppFooter.vue';
  import RawJsonModal from '@/components/RawJsonModal.vue';
  import SettingsModal from '@/components/SettingsModal.vue';
  import ToastContainer from '@/components/ToastContainer.vue';

  const route = useRoute();
  const appStore = useAppStore();
  const radarStore = useRadarStore();
  const insiderStore = useInsiderStore();

  let pollTimer: ReturnType<typeof setInterval> | null = null;

  const syncSettingsFromQuery = () => {
    const tab = route.query.settings;
    if (typeof tab === 'string' && tab) {
      appStore.openSettings(tab as SettingTabKey);
    }
  };

  const refreshActiveView = async () => {
    await appStore.fetchHealth();
    if (route.path.startsWith('/wallets')) {
      await Promise.all([
        insiderStore.fetchWallets(),
        insiderStore.fetchSyncStatus(),
        insiderStore.fetchAlertData(),
      ]);
      if (insiderStore.selectedWalletId) {
        await insiderStore.fetchWalletDetail(insiderStore.selectedWalletId);
      }
    } else {
      await Promise.all([
        radarStore.fetchFeed(),
        radarStore.fetchStats(),
        radarStore.fetchJobs(30),
      ]);
    }
  };

  const resetPolling = () => {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    if (appStore.autoRefresh) {
      pollTimer = setInterval(() => {
        refreshActiveView().catch(() => undefined);
      }, 30000);
    }
  };

  watch(
    () => route.query.settings,
    () => {
      syncSettingsFromQuery();
    }
  );

  watch(
    () => appStore.autoRefresh,
    () => {
      resetPolling();
    }
  );

  onMounted(async () => {
    await Promise.all([
      appStore.fetchHealth(),
      radarStore.fetchJobs(30),
      radarStore.fetchStats(),
      insiderStore.fetchWallets(),
    ]);
    syncSettingsFromQuery();
    resetPolling();
  });

  onUnmounted(() => {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  });
</script>
