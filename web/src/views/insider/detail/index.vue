<template>
  <div class="container">
    <Breadcrumb :items="['menu.insider', 'menu.insider.detail']" />
    <a-space direction="vertical" fill size="large">
      <a-page-header
        :title="wallet?.label || '钱包详情'"
        :subtitle="wallet?.address || '查看钱包持仓、交易和盈亏分析。'"
      >
        <template #extra>
          <a-space>
            <a-button size="small" @click="router.back()">返回</a-button>
            <a-radio-group v-model="period" type="button" @change="loadData">
              <a-radio value="7d">7天</a-radio>
              <a-radio value="30d">30天</a-radio>
              <a-radio value="all">全部</a-radio>
            </a-radio-group>
          </a-space>
        </template>
      </a-page-header>

      <div class="section-header">
        <div>
          <a-typography-title :heading="6" class="section-title">
            核心指标
          </a-typography-title>
          <a-typography-text type="secondary">
            聚焦钱包近期表现、成本结构和交易活跃度。
          </a-typography-text>
        </div>
      </div>

      <a-grid :cols="24" :col-gap="16" :row-gap="16">
        <a-grid-item :span="{ xs: 24, sm: 12, lg: 4 }">
          <a-card class="stat-card" :bordered="false"
            ><a-statistic
              title="胜率"
              :value="analytics?.win_rate || 0"
              :precision="1"
              suffix="%"
          /></a-card>
        </a-grid-item>
        <a-grid-item :span="{ xs: 24, sm: 12, lg: 4 }">
          <a-card class="stat-card" :bordered="false"
            ><a-statistic
              title="总盈亏"
              :value="analytics?.total_pnl || 0"
              :precision="4"
          /></a-card>
        </a-grid-item>
        <a-grid-item :span="{ xs: 24, sm: 12, lg: 4 }">
          <a-card class="stat-card" :bordered="false"
            ><a-statistic
              title="已实现盈亏"
              :value="analytics?.realized_pnl || 0"
              :precision="4"
          /></a-card>
        </a-grid-item>
        <a-grid-item :span="{ xs: 24, sm: 12, lg: 4 }">
          <a-card class="stat-card" :bordered="false"
            ><a-statistic
              title="未实现盈亏"
              :value="analytics?.unrealized_pnl || 0"
              :precision="4"
          /></a-card>
        </a-grid-item>
        <a-grid-item :span="{ xs: 24, sm: 12, lg: 4 }">
          <a-card class="stat-card" :bordered="false"
            ><a-statistic title="交易次数" :value="analytics?.tx_count || 0"
          /></a-card>
        </a-grid-item>
        <a-grid-item :span="{ xs: 24, sm: 12, lg: 4 }">
          <a-card class="stat-card" :bordered="false"
            ><a-statistic
              title="总成本"
              :value="analytics?.total_cost || 0"
              :precision="4"
          /></a-card>
        </a-grid-item>
      </a-grid>

      <a-card class="general-card detail-card" :bordered="false">
        <a-tabs default-active-key="holdings" type="rounded">
          <a-tab-pane key="holdings" title="持仓">
            <a-table
              :data="holdings"
              :loading="loading"
              row-key="mint_address"
              :pagination="{ pageSize: 20 }"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="代币" data-index="token_name">
                  <template #cell="{ record }">{{
                    record.token_name || shortAddress(record.mint_address)
                  }}</template>
                </a-table-column>
                <a-table-column title="数量" data-index="balance">
                  <template #cell="{ record }">{{
                    number(record.balance, 6)
                  }}</template>
                </a-table-column>
                <a-table-column title="Mint 地址" data-index="mint_address">
                  <template #cell="{ record }"
                    ><a-typography-text copyable>{{
                      record.mint_address
                    }}</a-typography-text></template
                  >
                </a-table-column>
                <a-table-column
                  title="当前价值(USD)"
                  data-index="current_value"
                >
                  <template #cell="{ record }"
                    >${{
                      number(record.current_value || record.usd_value, 2)
                    }}</template
                  >
                </a-table-column>
                <a-table-column title="盈亏" data-index="pnl">
                  <template #cell="{ record }">
                    <a-tag :color="record.pnl >= 0 ? 'green' : 'red'">{{
                      number(record.pnl, 4)
                    }}</a-tag>
                  </template>
                </a-table-column>
              </template>
            </a-table>
          </a-tab-pane>
          <a-tab-pane key="transactions" title="交易记录">
            <a-table
              :data="transactions"
              row-key="id"
              :pagination="{ pageSize: 20 }"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="时间" data-index="block_time">
                  <template #cell="{ record }">{{
                    formatTime(record.block_time)
                  }}</template>
                </a-table-column>
                <a-table-column title="类型" data-index="tx_type">
                  <template #cell="{ record }"
                    ><a-tag>{{ record.tx_type }}</a-tag></template
                  >
                </a-table-column>
                <a-table-column title="代币" data-index="token_name">
                  <template #cell="{ record }">{{
                    record.token_name || shortAddress(record.mint_address)
                  }}</template>
                </a-table-column>
                <a-table-column title="数量" data-index="amount">
                  <template #cell="{ record }">{{
                    number(record.amount, 4)
                  }}</template>
                </a-table-column>
                <a-table-column title="SOL 金额" data-index="sol_amount">
                  <template #cell="{ record }">{{
                    number(record.sol_amount, 4)
                  }}</template>
                </a-table-column>
              </template>
            </a-table>
          </a-tab-pane>
        </a-tabs>
      </a-card>
    </a-space>
  </div>
</template>

<script lang="ts" setup>
  import { onMounted, ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import {
    queryInsiderAnalytics,
    queryInsiderPortfolio,
    queryInsiderTransactions,
    queryInsiderWallet,
    type InsiderTransaction,
    type InsiderWallet,
    type TokenHolding,
    type WalletAnalytics,
  } from '@/api/insider';

  const route = useRoute();
  const router = useRouter();
  const wallet = ref<InsiderWallet>();
  const holdings = ref<TokenHolding[]>([]);
  const transactions = ref<InsiderTransaction[]>([]);
  const analytics = ref<WalletAnalytics>();
  const period = ref('all');
  const loading = ref(false);
  const walletId = Number(route.params.id);

  const shortAddress = (value: string) =>
    `${value.slice(0, 6)}...${value.slice(-6)}`;
  const number = (value: number, precision: number) =>
    Number(value || 0).toFixed(precision);
  const formatTime = (value: string) =>
    value ? new Date(value).toLocaleString() : '-';

  const loadData = async () => {
    loading.value = true;
    try {
      const [walletRes, holdingsRes, txRes, analyticsRes] = await Promise.all([
        queryInsiderWallet(walletId),
        queryInsiderPortfolio(walletId),
        queryInsiderTransactions(walletId),
        queryInsiderAnalytics(walletId, period.value),
      ]);
      wallet.value = walletRes.data.item;
      holdings.value = holdingsRes.data.items;
      transactions.value = txRes.data.items;
      analytics.value = analyticsRes.data.item;
    } finally {
      loading.value = false;
    }
  };

  onMounted(loadData);
</script>

<style scoped lang="less">
  .container {
    padding: 0 20px 20px 20px;
  }

  :deep(.arco-page-header) {
    padding-right: 0;
    padding-left: 0;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section-title {
    margin-bottom: 4px;
  }

  .stat-card {
    height: 100%;
  }

  .stat-card :deep(.arco-card-body) {
    padding: 20px;
  }

  .detail-card :deep(.arco-tabs-content) {
    padding-top: 4px;
  }
</style>
