<template>
  <aside
    class="w-full md:w-[345px] lg:w-[385px] shrink-0 border-l border-slate-200 h-full flex flex-col bg-white overflow-y-auto"
  >
    <div
      v-if="radarStore.loadingToken && !detail"
      class="p-8 text-center text-xs text-slate-400"
    >
      正在加载标的详情...
    </div>
    <div
      v-else-if="!detail"
      class="p-8 text-center text-xs text-slate-400 flex-1 flex items-center justify-center"
    >
      点击左侧观察名单或中间信号行查看标的详情与历史快照
    </div>

    <template v-else>
      <!-- 上半块：基础信息 + 观察控制 + 状态概览 -->
      <div class="p-4 border-b border-slate-200 shrink-0 space-y-3">
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-base font-bold text-slate-900">
                {{ detail.token.symbol || shortAddress(detail.token.address) }}
              </span>
              <span class="text-xs text-slate-500 truncate">
                {{ detail.token.name }}
              </span>
              <span
                class="bg-blue-50 text-blue-700 border border-blue-200 text-[10px] font-mono font-bold px-1.5 py-0.5 rounded"
              >
                {{ chainLabel(detail.token.chain) }}
              </span>
            </div>
            <div
              v-if="detail.token.narrative_theme"
              class="text-[11px] text-slate-500 mt-0.5"
            >
              叙事主题:
              <strong class="text-slate-800">{{
                detail.token.narrative_theme
              }}</strong>
            </div>
          </div>

          <button
            v-if="!watchItem"
            type="button"
            class="px-2.5 py-1 rounded-md text-[11px] font-semibold bg-[#09182b] hover:bg-slate-800 text-white shrink-0 cursor-pointer"
            @click="addWatch()"
          >
            + 加入观察
          </button>
          <button
            v-else
            type="button"
            class="px-2 py-1 rounded-md text-[11px] font-semibold bg-amber-50 text-amber-800 border border-amber-200 shrink-0 cursor-pointer"
            title="点击暂停观察"
            @click="pauseWatch()"
          >
            ★ 已观察
          </button>
        </div>

        <!-- 已在观察名单时：状态下拉 + 备注编辑 -->
        <div
          v-if="watchItem"
          class="bg-slate-50 border border-slate-200 rounded-lg p-2.5 space-y-2 text-xs"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="text-[11px] text-slate-500 font-semibold"
              >观察状态</span
            >
            <select
              v-model="watchStatusDraft"
              class="border border-slate-200 rounded px-2 py-1 bg-white text-xs font-semibold"
              @change="saveWatchChanges()"
            >
              <option
                v-for="opt in WATCHLIST_STATUS_OPTIONS"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </option>
            </select>
          </div>
          <div class="flex items-center gap-1.5">
            <input
              v-model="watchNoteDraft"
              type="text"
              placeholder="编辑观察备注..."
              class="flex-1 border border-slate-200 rounded px-2 py-1 text-xs bg-white focus:outline-none focus:border-blue-500"
            />
            <button
              type="button"
              class="px-2.5 py-1 bg-[#09182b] text-white rounded text-[11px] font-semibold cursor-pointer"
              @click="saveWatchChanges()"
            >
              保存备注
            </button>
          </div>
        </div>

        <!-- 地址复制行 -->
        <div
          class="flex items-center justify-between bg-slate-50 px-2.5 py-1.5 rounded-md border border-slate-100 text-[11px]"
        >
          <span class="text-slate-400">
            {{
              detail.token.chain === 'binance_perp' ? '合约交易对:' : '地址:'
            }}
          </span>
          <div class="flex items-center gap-1.5 font-mono text-slate-700">
            <span>{{ shortAddress(detail.token.address) }}</span>
            <button
              type="button"
              class="text-blue-600 hover:underline font-sans cursor-pointer"
              @click="copyAddress()"
            >
              复制
            </button>
          </div>
        </div>

        <!-- 4 项概览 -->
        <div
          class="grid grid-cols-4 gap-2 py-2 border-y border-slate-100 text-center"
        >
          <div>
            <div class="text-[10px] text-slate-400">相关信号</div>
            <div class="text-sm font-mono font-bold text-slate-900 mt-0.5">
              {{ detail.signals.length }}
            </div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400">快照数量</div>
            <div class="text-sm font-mono font-bold text-slate-900 mt-0.5">
              {{ detail.snapshots.length }}
            </div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400">最高分数</div>
            <div class="text-sm font-mono font-bold text-rose-600 mt-0.5">
              {{ highestScore }}
            </div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400">观察状态</div>
            <div
              class="text-xs font-bold mt-1"
              :class="watchItem ? 'text-emerald-600' : 'text-slate-400'"
            >
              {{ watchItem ? watchItem.status.toUpperCase() : '未加入' }}
            </div>
          </div>
        </div>

        <!-- GoPlus 安全标签（仅从 S5 raw_json.safety 解析） -->
        <div v-if="safetyInfo" class="flex flex-wrap gap-1.5 text-[10px]">
          <span
            class="px-2 py-0.5 rounded font-semibold border"
            :class="
              safetyInfo.safe && !safetyInfo.honeypot
                ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                : 'bg-rose-50 text-rose-700 border-rose-200'
            "
          >
            GoPlus:
            {{ safetyInfo.honeypot ? '高危貔貅' : '可正常交易' }}
          </span>
          <span
            class="px-2 py-0.5 rounded font-mono border"
            :class="
              safetyInfo.sellTax > 0
                ? 'bg-amber-50 text-amber-800 border-amber-200'
                : 'bg-slate-50 text-slate-600 border-slate-200'
            "
          >
            买税 {{ (safetyInfo.buyTax * 100).toFixed(1) }}% / 卖税
            {{ (safetyInfo.sellTax * 100).toFixed(1) }}%
          </span>
          <span
            v-if="safetyInfo.mintable"
            class="px-2 py-0.5 rounded bg-amber-50 text-amber-700 border border-amber-200 font-semibold"
          >
            可增发
          </span>
        </div>

        <!-- 叙事标签与社交链接 -->
        <div
          v-if="narrativeTags.length > 0 || socialLinks.length > 0"
          class="flex flex-wrap gap-1"
        >
          <span
            v-for="tag in narrativeTags"
            :key="tag"
            class="text-[10px] px-2 py-0.5 rounded bg-slate-100 text-slate-700 font-medium"
          >
            {{ tag }}
          </span>
          <span
            v-for="link in socialLinks"
            :key="link.key"
            class="text-[10px] px-2 py-0.5 rounded bg-orange-50 text-orange-700 border border-orange-200 font-mono"
          >
            {{ link.key }}: {{ link.value }}
          </span>
        </div>
        <div
          v-if="detail.token.description"
          class="text-[11px] text-slate-500 leading-relaxed"
        >
          {{ detail.token.description }}
        </div>
      </div>

      <!-- 下半块：历史快照与标的信号 -->
      <div class="p-4 flex-1 flex flex-col space-y-3">
        <div
          class="flex items-center justify-between border-b border-slate-100 pb-2"
        >
          <div class="flex items-center gap-3">
            <button
              type="button"
              class="text-xs font-bold uppercase tracking-wider cursor-pointer"
              :class="
                activeTab === 'snapshots'
                  ? 'text-slate-900 border-b-2 border-slate-900 pb-1 -mb-2.5'
                  : 'text-slate-400 hover:text-slate-700'
              "
              @click="activeTab = 'snapshots'"
            >
              历史快照 ({{ detail.snapshots.length }})
            </button>
            <button
              type="button"
              class="text-xs font-bold uppercase tracking-wider cursor-pointer"
              :class="
                activeTab === 'signals'
                  ? 'text-slate-900 border-b-2 border-slate-900 pb-1 -mb-2.5'
                  : 'text-slate-400 hover:text-slate-700'
              "
              @click="activeTab = 'signals'"
            >
              标的信号 ({{ detail.signals.length }})
            </button>
          </div>

          <div
            v-if="activeTab === 'snapshots' && snapshotSources.length > 1"
            class="flex items-center gap-1"
          >
            <button
              v-for="src in ['all', ...snapshotSources]"
              :key="src"
              type="button"
              class="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold cursor-pointer"
              :class="
                radarStore.snapshotSource === src
                  ? 'bg-[#09182b] text-white'
                  : 'bg-slate-100 text-slate-500'
              "
              @click="radarStore.snapshotSource = src"
            >
              {{ src.toUpperCase() }}
            </button>
          </div>
        </div>

        <!-- 历史快照列表（永续与链上代币展示不同字段） -->
        <div v-if="activeTab === 'snapshots'" class="space-y-1.5">
          <div
            v-if="visibleSnapshots.length === 0"
            class="py-6 text-center text-xs text-slate-400"
          >
            暂无历史快照
          </div>
          <div
            v-for="sn in visibleSnapshots"
            :key="sn.id"
            class="p-2.5 rounded-md border border-slate-100 bg-slate-50/70 text-[11px] space-y-1"
          >
            <div class="flex items-center justify-between font-mono">
              <span class="text-slate-500">
                {{ formatTime(sn.created_at) }} ·
                <strong class="text-slate-700">{{
                  sn.source.toUpperCase()
                }}</strong>
              </span>
              <span class="font-bold text-slate-900">{{
                formatPrice(sn.price)
              }}</span>
            </div>

            <!-- 永续合约字段 -->
            <div
              v-if="isPerp"
              class="grid grid-cols-3 gap-1 text-[10.5px] font-mono text-slate-600"
            >
              <span>量: {{ formatCompactUnit(sn.volume) }}</span>
              <span
                :class="
                  (sn.funding_pct ?? 0) < 0
                    ? 'text-rose-600'
                    : 'text-emerald-600'
                "
              >
                费率:
                {{
                  sn.funding_pct != null
                    ? `${Number(sn.funding_pct).toFixed(4)}%`
                    : '-'
                }}
              </span>
              <span>
                OI: {{ formatCompactUnit(sn.oi_usd) }}
                <strong
                  v-if="sn.oi_d6h != null"
                  :class="sn.oi_d6h >= 0 ? 'text-emerald-600' : 'text-rose-500'"
                >
                  ({{ sn.oi_d6h > 0 ? '+' : ''
                  }}{{ Number(sn.oi_d6h).toFixed(2) }}%)
                </strong>
              </span>
            </div>

            <!-- 链上代币字段 -->
            <div
              v-else
              class="flex items-center justify-between text-[10.5px] font-mono text-slate-500"
            >
              <span>
                MC: {{ formatCompactUnit(sn.mc) }} · Liq:
                {{ formatCompactUnit(sn.liq) }}
              </span>
              <span>
                1h 买{{ sn.buys_1h ?? 0 }}/卖{{ sn.sells_1h ?? 0 }}
                <template v-if="sn.smart_money != null">
                  · SM {{ sn.smart_money }}
                </template>
              </span>
            </div>
          </div>
        </div>

        <!-- 标的信号列表 -->
        <div v-else class="space-y-2">
          <div
            v-if="detail.signals.length === 0"
            class="py-6 text-center text-xs text-slate-400"
          >
            暂无相关信号
          </div>
          <div
            v-for="rs in detail.signals"
            :key="rs.id || rs.created_at"
            class="p-2.5 rounded-lg border border-slate-200 bg-slate-50/60 hover:bg-blue-50/40 cursor-pointer space-y-1"
            @click="
              appStore.openRawModal(
                `${detail.token.symbol} · 信号报文`,
                rs.source.toUpperCase(),
                rs.raw_json
              )
            "
          >
            <div class="flex items-center justify-between">
              <span class="font-bold text-slate-900 text-xs">
                {{ signalTypeLabel(rs.signal_type, rs.reason, rs.raw_json) }}
              </span>
              <span class="font-mono text-[11px] font-bold text-rose-600">
                {{ Number(rs.score || 0).toFixed(0) }} 分 ({{ rs.priority }})
              </span>
            </div>
            <div class="text-[11px] text-slate-600 leading-snug">
              {{ summarizeReason(rs.signal_type, rs.reason) }}
            </div>
            <div
              class="flex items-center justify-between text-[10px] font-mono text-slate-400 pt-0.5"
            >
              <span>来源: {{ sourceLabel(rs.source) }}</span>
              <span>{{ formatTime(rs.created_at) }} · 查看报文</span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </aside>
</template>

<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { useAppStore, useRadarStore } from '@/store';
  import { WATCHLIST_STATUS_OPTIONS } from '@/constants/radar';
  import {
    chainLabel,
    extractSafetyFromSignals,
    formatCompactUnit,
    formatPrice,
    formatTime,
    parseJsonObject,
    shortAddress,
    signalTypeLabel,
    sourceLabel,
    summarizeReason,
  } from '@/utils/format';

  const appStore = useAppStore();
  const radarStore = useRadarStore();

  const activeTab = ref<'snapshots' | 'signals'>('snapshots');
  const watchStatusDraft = ref<'watch' | 'active' | 'paused'>('active');
  const watchNoteDraft = ref('');

  const detail = computed(() => radarStore.tokenDetail);
  const watchItem = computed(() => detail.value?.watch_item || null);
  const isPerp = computed(() => detail.value?.token.chain === 'binance_perp');

  watch(
    watchItem,
    (item) => {
      if (item) {
        watchStatusDraft.value =
          (item.status as 'watch' | 'active' | 'paused') || 'watch';
        watchNoteDraft.value = item.note || '';
      }
    },
    { immediate: true }
  );

  const highestScore = computed(() => {
    const list = detail.value?.signals || [];
    if (list.length === 0) return 0;
    return Math.max(...list.map((s) => Number(s.score || 0)));
  });

  const safetyInfo = computed(() =>
    extractSafetyFromSignals(detail.value?.signals || [])
  );

  const narrativeTags = computed(() => {
    const raw = detail.value?.token.narrative_tags_json;
    if (!raw) return [] as string[];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch {
      return [];
    }
  });

  const socialLinks = computed(() => {
    const obj = parseJsonObject(detail.value?.token.social_links_json);
    return Object.entries(obj).map(([key, value]) => ({
      key,
      value: String(value),
    }));
  });

  const snapshotSources = computed(() => {
    const set = new Set(
      (detail.value?.snapshots || []).map((s) => s.source).filter(Boolean)
    );
    return Array.from(set);
  });

  const visibleSnapshots = computed(() => {
    const list = detail.value?.snapshots || [];
    if (radarStore.snapshotSource === 'all') return list;
    return list.filter((s) => s.source === radarStore.snapshotSource);
  });

  const copyAddress = async () => {
    const addr = detail.value?.token.address;
    if (!addr) return;
    await navigator.clipboard.writeText(addr);
    appStore.showToast(`已复制 ${shortAddress(addr)}`, 'success');
  };

  const addWatch = async () => {
    if (!detail.value) return;
    try {
      await radarStore.saveWatchItem({
        chain: detail.value.token.chain,
        address: detail.value.token.address,
        symbol: detail.value.token.symbol,
        name: detail.value.token.name,
        status: 'active',
      });
      appStore.showToast('已加入观察名单', 'success');
    } catch {
      appStore.showToast('加入观察名单失败', 'error');
    }
  };

  const pauseWatch = async () => {
    if (!watchItem.value) return;
    try {
      await radarStore.pauseWatchItem(watchItem.value);
      appStore.showToast('已暂停观察该标的', 'info');
    } catch {
      appStore.showToast('更新观察状态失败', 'error');
    }
  };

  const saveWatchChanges = async () => {
    if (!detail.value) return;
    try {
      await radarStore.saveWatchItem({
        chain: detail.value.token.chain,
        address: detail.value.token.address,
        symbol: detail.value.token.symbol,
        name: detail.value.token.name,
        status: watchStatusDraft.value,
        note: watchNoteDraft.value.trim(),
      });
      appStore.showToast('观察配置已更新', 'success');
    } catch {
      appStore.showToast('保存观察配置失败', 'error');
    }
  };
</script>
