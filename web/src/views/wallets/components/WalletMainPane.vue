<template>
  <section
    class="flex-1 h-full min-w-0 flex flex-col bg-[#f3f5f8] p-3 md:p-4 gap-3 overflow-y-auto"
  >
    <div
      v-if="!wallet"
      class="flex-1 bg-white border border-slate-200 rounded-xl flex items-center justify-center text-xs text-slate-400"
    >
      请从左侧选择一个监控钱包查看持仓与交易记录
    </div>

    <template v-else>
      <!-- 钱包头部信息卡 -->
      <div
        class="bg-white border border-slate-200 rounded-xl p-4 flex flex-wrap items-center justify-between gap-3 shrink-0"
      >
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h2 class="text-base font-bold text-slate-900 truncate">
              {{ wallet.label || shortAddress(wallet.address) }}
            </h2>
            <span
              class="bg-purple-50 text-purple-700 border border-purple-200 text-[10px] font-bold px-1.5 py-0.5 rounded"
            >
              SOLANA
            </span>
          </div>
          <div
            class="flex items-center gap-2 mt-1 text-xs font-mono text-slate-500"
          >
            <span class="truncate">{{ wallet.address }}</span>
            <button
              type="button"
              class="text-blue-600 hover:underline font-sans cursor-pointer shrink-0"
              @click="copyAddress()"
            >
              复制地址
            </button>
            <a
              :href="`https://solscan.io/account/${wallet.address}`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-blue-600 hover:underline font-sans shrink-0"
            >
              Solscan ↗
            </a>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <button
            type="button"
            class="px-3 py-1.5 border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 rounded-md text-xs font-semibold cursor-pointer"
            @click="insiderStore.openEditModal(wallet)"
          >
            编辑备注
          </button>

          <button
            v-if="!confirmDelete"
            type="button"
            class="px-3 py-1.5 border border-rose-200 bg-white hover:bg-rose-50 text-rose-600 rounded-md text-xs font-semibold cursor-pointer"
            @click="confirmDelete = true"
          >
            删除钱包
          </button>
          <div v-else class="flex items-center gap-1.5">
            <button
              type="button"
              class="px-2.5 py-1.5 bg-rose-600 hover:bg-rose-700 text-white rounded-md text-xs font-semibold cursor-pointer"
              @click="handleDelete()"
            >
              确认删除
            </button>
            <button
              type="button"
              class="px-2.5 py-1.5 border border-slate-200 bg-white text-slate-600 rounded-md text-xs cursor-pointer"
              @click="confirmDelete = false"
            >
              取消
            </button>
          </div>
        </div>
      </div>

      <!-- 当前持仓卡 -->
      <div
        class="bg-white border border-slate-200 rounded-xl overflow-hidden shrink-0"
      >
        <div
          class="px-4 py-3 border-b border-slate-100 flex items-center justify-between"
        >
          <span
            class="text-xs font-bold uppercase tracking-wider text-slate-800"
          >
            当前持仓 ({{ insiderStore.holdings.length }})
          </span>
          <span class="text-[11px] font-mono text-slate-500">
            总估值:
            <strong class="text-slate-900">{{
              formatUsd(wallet.balanceUsd)
            }}</strong>
          </span>
        </div>

        <div class="overflow-x-auto max-h-72 overflow-y-auto">
          <table class="w-full text-left border-collapse text-xs">
            <thead class="sticky top-0 bg-slate-50">
              <tr
                class="border-b border-slate-100 text-[11px] text-slate-400 font-semibold"
              >
                <th class="py-2 px-4">代币</th>
                <th class="py-2 px-4 text-right">余额</th>
                <th class="py-2 px-4">Mint 地址</th>
                <th class="py-2 px-4 text-right">成本基准</th>
                <th class="py-2 px-4 text-right">当前价值</th>
                <th class="py-2 px-4 text-right">盈亏</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              <tr v-if="insiderStore.holdings.length === 0">
                <td colspan="6" class="py-8 text-center text-slate-400">
                  暂无持仓数据
                </td>
              </tr>
              <tr
                v-for="h in insiderStore.holdings"
                :key="h.mint_address"
                class="hover:bg-slate-50/80"
              >
                <td class="py-2.5 px-4 font-bold text-slate-900">
                  {{ h.token_name || shortAddress(h.mint_address) }}
                </td>
                <td class="py-2.5 px-4 text-right font-mono">
                  {{ Number(h.balance || 0).toLocaleString() }}
                </td>
                <td class="py-2.5 px-4 font-mono text-slate-400">
                  {{ shortAddress(h.mint_address) }}
                </td>
                <td class="py-2.5 px-4 text-right font-mono text-slate-500">
                  {{ formatUsd(h.cost_basis) }}
                </td>
                <td
                  class="py-2.5 px-4 text-right font-mono font-bold text-slate-900"
                >
                  {{ formatUsd(h.current_value || h.usd_value) }}
                </td>
                <td
                  class="py-2.5 px-4 text-right font-mono font-bold"
                  :class="h.pnl >= 0 ? 'text-emerald-600' : 'text-rose-500'"
                >
                  {{ formatUsd(h.pnl, true) }}
                  <span class="text-[10px] font-normal">
                    ({{ Number(h.pnl_percent || 0).toFixed(1) }}%)
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 交易记录卡（上下同时显示） -->
      <div
        class="bg-white border border-slate-200 rounded-xl overflow-hidden flex-1 min-h-[260px] flex flex-col"
      >
        <div
          class="px-4 py-3 border-b border-slate-100 flex items-center justify-between shrink-0"
        >
          <span
            class="text-xs font-bold uppercase tracking-wider text-slate-800"
          >
            最近交易记录 ({{ insiderStore.transactions.length }})
          </span>
        </div>

        <div class="flex-1 overflow-x-auto overflow-y-auto">
          <table class="w-full text-left border-collapse text-xs">
            <thead class="sticky top-0 bg-slate-50">
              <tr
                class="border-b border-slate-100 text-[11px] text-slate-400 font-semibold"
              >
                <th class="py-2 px-4">时间</th>
                <th class="py-2 px-4">类型</th>
                <th class="py-2 px-4">代币</th>
                <th class="py-2 px-4 text-right">数量</th>
                <th class="py-2 px-4 text-right">SOL 金额</th>
                <th class="py-2 px-4 text-right">交易签名</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              <tr v-if="insiderStore.transactions.length === 0">
                <td colspan="6" class="py-8 text-center text-slate-400">
                  暂无交易记录
                </td>
              </tr>
              <tr
                v-for="tx in insiderStore.transactions"
                :key="tx.id"
                class="hover:bg-slate-50/80"
              >
                <td
                  class="py-2.5 px-4 font-mono text-slate-500 whitespace-nowrap"
                >
                  {{ formatTime(tx.block_time) }}
                </td>
                <td class="py-2.5 px-4 font-bold whitespace-nowrap">
                  <span
                    :class="
                      tx.tx_type.toLowerCase().includes('buy')
                        ? 'text-emerald-600'
                        : tx.tx_type.toLowerCase().includes('sell')
                        ? 'text-rose-500'
                        : 'text-slate-700'
                    "
                  >
                    {{ tx.tx_type.toUpperCase() }}
                  </span>
                </td>
                <td class="py-2.5 px-4 font-bold text-slate-900">
                  {{ tx.token_name || shortAddress(tx.mint_address) }}
                </td>
                <td class="py-2.5 px-4 text-right font-mono">
                  {{ Number(tx.amount || 0).toLocaleString() }}
                </td>
                <td
                  class="py-2.5 px-4 text-right font-mono font-semibold text-slate-800"
                >
                  {{ Number(tx.sol_amount || 0).toFixed(4) }} SOL
                </td>
                <td class="py-2.5 px-4 text-right font-mono text-slate-400">
                  <a
                    :href="`https://solscan.io/tx/${tx.signature}`"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="hover:text-blue-600 hover:underline"
                  >
                    {{ shortAddress(tx.signature) }} ↗
                  </a>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { useAppStore, useInsiderStore } from '@/store';
  import { formatTime, formatUsd, shortAddress } from '@/utils/format';

  const appStore = useAppStore();
  const insiderStore = useInsiderStore();

  const confirmDelete = ref(false);
  const wallet = computed(() => insiderStore.selectedWallet);

  watch(wallet, () => {
    confirmDelete.value = false;
  });

  const copyAddress = async () => {
    if (!wallet.value) return;
    await navigator.clipboard.writeText(wallet.value.address);
    appStore.showToast('已复制钱包地址', 'success');
  };

  const handleDelete = async () => {
    if (!wallet.value) return;
    try {
      await insiderStore.removeWallet(wallet.value.id);
      confirmDelete.value = false;
      appStore.showToast('监控钱包已删除', 'success');
    } catch {
      appStore.showToast('删除钱包失败', 'error');
    }
  };
</script>
