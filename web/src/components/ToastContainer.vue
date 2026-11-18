<template>
  <div
    v-if="appStore.toastVisible"
    class="fixed bottom-11 left-1/2 -translate-x-1/2 z-50 px-4 py-2 rounded-lg shadow-lg text-xs font-medium text-white transition-all"
    :class="bgClass"
  >
    {{ appStore.toastMessage }}
  </div>
</template>

<script setup lang="ts">
  import { computed } from 'vue';
  import { useAppStore } from '@/store';

  const appStore = useAppStore();

  const bgClass = computed(() => {
    if (appStore.toastType === 'error') return 'bg-rose-600';
    if (appStore.toastType === 'success') return 'bg-emerald-700';
    return 'bg-[#09182b]';
  });
</script>
