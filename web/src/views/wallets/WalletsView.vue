<template>
  <div class="flex-1 flex overflow-hidden min-h-0 w-full">
    <WalletSidebar
      :class="appStore.mobilePane === 'left' ? 'flex' : 'hidden md:flex'"
      @select-wallet="onSelectWallet"
    />
    <WalletMainPane
      :class="appStore.mobilePane === 'center' ? 'flex' : 'hidden md:flex'"
    />
    <WalletAnalyticsPane
      :class="appStore.mobilePane === 'right' ? 'flex' : 'hidden md:flex'"
    />
    <EditWalletModal />
  </div>
</template>

<script setup lang="ts">
  import { onMounted, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { useAppStore, useInsiderStore } from '@/store';
  import WalletSidebar from './components/WalletSidebar.vue';
  import WalletMainPane from './components/WalletMainPane.vue';
  import WalletAnalyticsPane from './components/WalletAnalyticsPane.vue';
  import EditWalletModal from './components/EditWalletModal.vue';

  const route = useRoute();
  const router = useRouter();
  const appStore = useAppStore();
  const insiderStore = useInsiderStore();

  const onSelectWallet = async (id: number) => {
    await insiderStore.selectWallet(id);
    router.replace(`/wallets/${id}`);
    if (window.innerWidth < 768) {
      appStore.applyMobilePane('center');
    }
  };

  watch(
    () => route.params.id,
    (idParam) => {
      const id = Number(idParam);
      if (id && id !== insiderStore.selectedWalletId) {
        insiderStore.selectWallet(id).catch(() => undefined);
      }
    }
  );

  onMounted(async () => {
    await Promise.all([
      insiderStore.fetchWallets(),
      insiderStore.fetchSyncStatus(),
      insiderStore.fetchAlertData(),
    ]);
    const idParam = Number(route.params.id);
    if (idParam) {
      await insiderStore.selectWallet(idParam);
    }
  });
</script>
