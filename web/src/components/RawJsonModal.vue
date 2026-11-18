<template>
  <div
    v-if="appStore.rawModalOpen"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
    @click.self="appStore.closeRawModal()"
  >
    <div
      class="bg-white rounded-xl shadow-2xl border border-slate-200 max-w-lg w-full overflow-hidden flex flex-col max-h-[88vh]"
    >
      <div
        class="p-4 border-b border-slate-100 flex items-center justify-between bg-slate-50/70"
      >
        <div class="flex items-center gap-2 min-w-0">
          <span class="font-bold text-slate-900 text-sm truncate">
            {{ appStore.rawModalTitle }}
          </span>
          <span
            class="bg-[#0f1d33] text-white text-[10px] font-mono font-bold px-2 py-0.5 rounded shrink-0"
          >
            {{ appStore.rawModalBadge }}
          </span>
        </div>
        <button
          type="button"
          class="w-7 h-7 rounded-md flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-200/50 cursor-pointer text-sm font-semibold"
          @click="appStore.closeRawModal()"
        >
          ✕
        </button>
      </div>

      <div class="p-4 overflow-y-auto text-xs">
        <pre
          class="bg-[#09182b] text-slate-100 p-3.5 rounded-lg font-mono text-[11px] leading-relaxed overflow-x-auto max-h-96"
          >{{ appStore.rawModalJson }}</pre
        >
      </div>

      <div
        class="p-3 bg-slate-50 border-t border-slate-100 flex items-center justify-end gap-2"
      >
        <button
          type="button"
          class="px-3 py-1.5 border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 font-medium rounded-md text-xs cursor-pointer"
          @click="copyJson"
        >
          复制 JSON
        </button>
        <button
          type="button"
          class="px-4 py-1.5 bg-[#09182b] hover:bg-slate-800 text-white font-medium rounded-md text-xs cursor-pointer"
          @click="appStore.closeRawModal()"
        >
          关闭
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { useAppStore } from '@/store';

  const appStore = useAppStore();

  const copyJson = async () => {
    try {
      await navigator.clipboard.writeText(appStore.rawModalJson);
      appStore.showToast('已复制 JSON 报文', 'success');
    } catch {
      appStore.showToast('复制失败，请手动选中复制', 'error');
    }
  };
</script>
