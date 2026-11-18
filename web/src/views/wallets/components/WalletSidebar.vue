<template>
  <aside
    class="w-full md:w-[260px] lg:w-[275px] shrink-0 border-r border-slate-200 h-full flex flex-col bg-white overflow-hidden"
  >
    <!-- 同步状态卡 -->
    <div class="p-3.5 border-b border-slate-200 bg-slate-50/60 shrink-0">
      <div class="flex items-center justify-between mb-1.5">
        <div class="flex items-center gap-1.5 text-xs font-bold text-slate-800">
          <span
            class="w-2 h-2 rounded-full"
            :class="
              insiderStore.syncStatus.syncing
                ? 'bg-blue-500 animate-pulse'
                : insiderStore.syncStatus.last_error
                ? 'bg-rose-500'
                : 'bg-emerald-500'
            "
          />
          <span>{{
            insiderStore.syncStatus.syncing ? '正在同步链上数据...' : '同步就绪'
          }}</span>
        </div>
        <button
          type="button"
          :disabled="insiderStore.syncStatus.syncing"
          class="px-2.5 py-1 bg-[#09182b] hover:bg-slate-800 disabled:opacity-50 text-white text-[11px] font-semibold rounded-md cursor-pointer"
          @click="handleSync()"
        >
          {{ insiderStore.syncStatus.syncing ? '同步中' : '立即同步' }}
        </button>
      </div>
      <div
        class="flex items-center justify-between text-[11px] text-slate-500 font-mono"
      >
        <span>引擎: {{ insiderStore.syncStatus.engine || 'service' }}</span>
        <span>{{ insiderStore.wallets.length }} 个钱包</span>
      </div>
      <div
        v-if="insiderStore.syncStatus.last_error"
        class="mt-1.5 text-[10.5px] text-rose-600 bg-rose-50 border border-rose-200 rounded px-2 py-1 break-all"
      >
        {{ insiderStore.syncStatus.last_error }}
      </div>
    </div>

    <!-- 监控钱包列表头 -->
    <div
      class="px-3.5 py-2.5 border-b border-slate-100 flex items-center justify-between shrink-0"
    >
      <span
        class="text-[11px] font-bold tracking-wider uppercase text-slate-800"
      >
        监控钱包 (Solana)
      </span>
      <button
        type="button"
        class="px-2 py-0.5 rounded text-[11px] font-semibold border border-slate-200 bg-white hover:bg-slate-100 text-slate-700 cursor-pointer"
        @click="addOpen = !addOpen"
      >
        {{ addOpen ? '收起' : '+ 添加' }}
      </button>
    </div>

    <!-- 添加钱包表单（Base58 校验） -->
    <form
      v-if="addOpen"
      class="p-3 border-b border-slate-200 bg-slate-50 space-y-2 text-xs shrink-0"
      @submit.prevent="submitAdd()"
    >
      <p v-if="addrError" class="text-[11px] text-rose-600 font-medium">
        {{ addrError }}
      </p>
      <input
        v-model="form.address"
        type="text"
        required
        placeholder="粘贴 Solana Base58 钱包地址..."
        class="w-full border border-slate-200 rounded-md px-2.5 py-1.5 font-mono text-[11px] bg-white focus:outline-none focus:border-blue-500"
        @input="addrError = ''"
      />
      <input
        v-model="form.label"
        type="text"
        placeholder="备注名称（可选）"
        class="w-full border border-slate-200 rounded-md px-2.5 py-1.5 text-xs bg-white focus:outline-none focus:border-blue-500"
      />
      <button
        type="submit"
        :disabled="submitting || !form.address.trim()"
        class="w-full bg-[#09182b] hover:bg-slate-800 disabled:bg-slate-300 text-white font-semibold py-1.5 rounded-md cursor-pointer"
      >
        {{ submitting ? '添加中...' : '确认添加钱包' }}
      </button>
    </form>

    <!-- 钱包列表（7d 资产、胜率、盈亏） -->
    <div class="flex-1 overflow-y-auto divide-y divide-slate-100">
      <div
        v-if="insiderStore.loadingWallets && insiderStore.wallets.length === 0"
        class="p-6 text-center text-xs text-slate-400"
      >
        加载监控钱包...
      </div>
      <div
        v-else-if="insiderStore.wallets.length === 0"
        class="p-6 text-center text-xs text-slate-400"
      >
        暂无监控钱包，点击上方「+ 添加」
      </div>
      <div
        v-for="w in insiderStore.wallets"
        :key="w.id"
        class="py-2.5 px-3 transition-colors cursor-pointer group"
        :class="
          insiderStore.selectedWalletId === w.id
            ? 'bg-blue-50/60'
            : 'hover:bg-slate-50/80'
        "
        @click="$emit('selectWallet', w.id)"
      >
        <div class="flex items-center justify-between mb-1">
          <span
            class="text-xs font-bold text-slate-900 truncate group-hover:text-blue-600"
          >
            {{ w.label || shortAddress(w.address) }}
          </span>
          <span
            class="bg-purple-50 text-purple-700 border border-purple-200 text-[9px] font-bold px-1.5 py-0.5 rounded shrink-0"
          >
            SOL
          </span>
        </div>

        <div class="text-[10.5px] text-slate-500 space-y-0.5 font-mono">
          <div class="flex items-center justify-between">
            <span class="font-sans">当前资产:</span>
            <strong class="text-slate-800">{{
              formatUsd(w.balanceUsd)
            }}</strong>
          </div>
          <div class="flex items-center justify-between">
            <span class="font-sans">7d 胜率 / 盈亏:</span>
            <span>
              <strong class="text-slate-700"
                >{{ w.winRate.toFixed(1) }}%</strong
              >
              ·
              <strong
                :class="w.totalPnl >= 0 ? 'text-emerald-600' : 'text-rose-500'"
              >
                {{ formatUsd(w.totalPnl, true) }}
              </strong>
            </span>
          </div>
        </div>

        <div class="mt-1 text-[10px] font-mono text-slate-400 truncate">
          {{ shortAddress(w.address) }}
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
  import { reactive, ref } from 'vue';
  import { useAppStore, useInsiderStore } from '@/store';
  import { formatUsd, isValidSolanaBase58, shortAddress } from '@/utils/format';

  defineEmits<{
    (e: 'selectWallet', id: number): void;
  }>();

  const appStore = useAppStore();
  const insiderStore = useInsiderStore();

  const addOpen = ref(false);
  const submitting = ref(false);
  const addrError = ref('');
  const form = reactive({
    address: '',
    label: '',
  });

  const handleSync = async () => {
    try {
      await insiderStore.triggerSync((err) => {
        if (err) appStore.showToast(`同步结束但有错误: ${err}`, 'error');
        else appStore.showToast('钱包数据同步完成', 'success');
      });
      appStore.showToast('已触发链上同步', 'info');
    } catch {
      appStore.showToast('触发同步失败', 'error');
    }
  };

  const submitAdd = async () => {
    const cleanAddr = form.address.trim();
    if (!isValidSolanaBase58(cleanAddr)) {
      addrError.value = '请输入有效的 Solana Base58 钱包地址（32~44 位字符）';
      return;
    }
    submitting.value = true;
    try {
      await insiderStore.addWallet(cleanAddr, form.label.trim());
      form.address = '';
      form.label = '';
      addOpen.value = false;
      appStore.showToast('监控钱包已添加', 'success');
    } catch {
      appStore.showToast('添加监控钱包失败', 'error');
    } finally {
      submitting.value = false;
    }
  };
</script>
