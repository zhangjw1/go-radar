<template>
  <div
    v-if="insiderStore.editModalOpen && insiderStore.editingWallet"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
    @click.self="insiderStore.closeEditModal()"
  >
    <div
      class="bg-white rounded-xl shadow-2xl border border-slate-200 max-w-sm w-full p-5 space-y-4"
    >
      <div
        class="flex items-center justify-between pb-2 border-b border-slate-100"
      >
        <h3 class="font-bold text-slate-900 text-sm">编辑钱包备注</h3>
        <button
          type="button"
          class="text-slate-400 hover:text-slate-600 text-sm font-semibold cursor-pointer"
          @click="insiderStore.closeEditModal()"
        >
          ✕
        </button>
      </div>

      <form class="space-y-3 text-xs" @submit.prevent="submit()">
        <div>
          <label class="block text-slate-500 font-medium mb-1">钱包地址</label>
          <input
            :value="insiderStore.editingWallet.address"
            disabled
            class="w-full border border-slate-200 rounded-md px-3 py-2 font-mono text-[11px] bg-slate-100 text-slate-500"
          />
        </div>
        <div>
          <label class="block text-slate-600 font-medium mb-1">备注名称</label>
          <input
            v-model="labelDraft"
            type="text"
            placeholder="输入钱包备注名称"
            class="w-full border border-slate-200 rounded-md px-3 py-2 text-xs focus:ring-1 focus:ring-blue-500 focus:outline-none"
          />
        </div>
        <div class="pt-2 flex items-center justify-end gap-2">
          <button
            type="button"
            class="px-3 py-1.5 rounded-md border border-slate-200 text-slate-600 hover:bg-slate-50 cursor-pointer"
            @click="insiderStore.closeEditModal()"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="saving"
            class="px-4 py-1.5 rounded-md bg-[#0c1a30] text-white font-medium hover:bg-[#152a4a] disabled:opacity-50 cursor-pointer"
          >
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { useAppStore, useInsiderStore } from '@/store';

  const appStore = useAppStore();
  const insiderStore = useInsiderStore();

  const labelDraft = ref('');
  const saving = ref(false);

  watch(
    () => insiderStore.editingWallet,
    (w) => {
      labelDraft.value = w?.label || '';
    },
    { immediate: true }
  );

  const submit = async () => {
    if (!insiderStore.editingWallet) return;
    saving.value = true;
    try {
      await insiderStore.renameWallet(
        insiderStore.editingWallet.id,
        labelDraft.value.trim()
      );
      insiderStore.closeEditModal();
      appStore.showToast('钱包备注已更新', 'success');
    } catch {
      appStore.showToast('更新钱包备注失败', 'error');
    } finally {
      saving.value = false;
    }
  };
</script>
