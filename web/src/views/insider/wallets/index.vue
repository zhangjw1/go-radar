<template>
  <div class="container">
    <Breadcrumb :items="['menu.insider', 'menu.insider.wallets']" />
    <a-space direction="vertical" fill size="large">
      <a-page-header
        title="钱包监控"
        subtitle="跟踪钱包持仓、盈亏表现和同步状态。"
      >
        <template #extra>
          <a-space>
            <a-button :loading="syncing" @click="handleSync">立即同步</a-button>
            <a-button type="primary" @click="openCreate">添加钱包</a-button>
          </a-space>
        </template>
      </a-page-header>

      <div class="section-header">
        <div>
          <a-typography-title :heading="6" class="section-title">
            钱包概览
          </a-typography-title>
          <a-typography-text type="secondary">
            快速查看重点钱包的资产与盈亏概况。
          </a-typography-text>
        </div>
      </div>

      <a-grid :cols="24" :col-gap="16" :row-gap="16">
        <a-grid-item
          v-for="wallet in walletCards"
          :key="wallet.id"
          :span="{ xs: 24, sm: 12, lg: 8, xl: 6 }"
        >
          <a-card
            hoverable
            :bordered="false"
            class="wallet-card summary-card"
            @click="goDetail(wallet.id)"
          >
            <a-space direction="vertical" fill>
              <a-row justify="space-between" align="center">
                <a-typography-text bold>{{
                  wallet.label || shortAddress(wallet.address)
                }}</a-typography-text>
                <a-tag color="arcoblue">{{
                  shortAddress(wallet.address)
                }}</a-tag>
              </a-row>
              <a-statistic
                title="当前资产(USD)"
                :value="wallet.balance"
                :precision="2"
                show-group-separator
              />
              <a-row>
                <a-col :span="12">
                  <a-statistic
                    title="胜率"
                    :value="wallet.analytics?.win_rate || 0"
                    :precision="1"
                    suffix="%"
                  />
                </a-col>
                <a-col :span="12">
                  <a-statistic
                    title="总盈亏"
                    :value="wallet.analytics?.total_pnl || 0"
                    :precision="4"
                  />
                </a-col>
              </a-row>
            </a-space>
          </a-card>
        </a-grid-item>
      </a-grid>

      <a-card class="general-card" :bordered="false">
        <template #title>钱包列表</template>
        <template #extra>
          <a-tag color="arcoblue">{{ wallets.length }} 个钱包</a-tag>
        </template>
        <a-table
          :data="wallets"
          :loading="loading"
          row-key="id"
          :pagination="false"
          :bordered="false"
        >
          <template #columns>
            <a-table-column title="备注" data-index="label">
              <template #cell="{ record }">{{ record.label || '-' }}</template>
            </a-table-column>
            <a-table-column title="钱包地址" data-index="address">
              <template #cell="{ record }">
                <a-typography-text copyable>{{
                  record.address
                }}</a-typography-text>
              </template>
            </a-table-column>
            <a-table-column title="创建时间" data-index="created_at">
              <template #cell="{ record }">{{
                formatTime(record.created_at)
              }}</template>
            </a-table-column>
            <a-table-column title="操作">
              <template #cell="{ record }">
                <a-space>
                  <a-button size="mini" @click.stop="goDetail(record.id)"
                    >查看详情</a-button
                  >
                  <a-button size="mini" @click.stop="openEdit(record)"
                    >编辑</a-button
                  >
                  <a-popconfirm
                    content="确认删除这个钱包吗？"
                    @ok="handleDelete(record.id)"
                  >
                    <a-button size="mini" status="danger" @click.stop
                      >删除</a-button
                    >
                  </a-popconfirm>
                </a-space>
              </template>
            </a-table-column>
          </template>
        </a-table>
      </a-card>
    </a-space>

    <a-modal
      v-model:visible="modalVisible"
      :title="editing ? '编辑钱包' : '添加钱包'"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item v-if="!editing" field="address" label="钱包地址">
          <a-input
            v-model="form.address"
            placeholder="请输入 Solana 钱包地址"
          />
        </a-form-item>
        <a-form-item field="label" label="备注">
          <a-input v-model="form.label" placeholder="请输入备注名称" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, onMounted, reactive, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { Message } from '@arco-design/web-vue';
  import {
    createInsiderWallet,
    deleteInsiderWallet,
    queryInsiderAnalytics,
    queryInsiderPortfolio,
    queryInsiderSyncStatus,
    queryInsiderWallets,
    triggerInsiderSync,
    updateInsiderWallet,
    type InsiderWallet,
    type WalletAnalytics,
  } from '@/api/insider';

  const router = useRouter();
  const wallets = ref<InsiderWallet[]>([]);
  const analyticsMap = ref<Record<number, WalletAnalytics>>({});
  const balanceMap = ref<Record<number, number>>({});
  const loading = ref(false);
  const syncing = ref(false);
  const modalVisible = ref(false);
  const editing = ref<InsiderWallet | null>(null);
  const form = reactive({ address: '', label: '' });

  const walletCards = computed(() =>
    wallets.value.map((wallet) => ({
      ...wallet,
      balance: balanceMap.value[wallet.id] || 0,
      analytics: analyticsMap.value[wallet.id],
    }))
  );

  const shortAddress = (value: string) =>
    `${value.slice(0, 6)}...${value.slice(-6)}`;
  const formatTime = (value: string) =>
    value ? new Date(value).toLocaleString() : '-';
  const wait = (ms: number) =>
    new Promise((resolve) => {
      setTimeout(resolve, ms);
    });

  const loadData = async () => {
    loading.value = true;
    try {
      const { data } = await queryInsiderWallets();
      wallets.value = data.items;
      const analytics: Record<number, WalletAnalytics> = {};
      const balances: Record<number, number> = {};
      await Promise.all(
        data.items.map(async (wallet) => {
          const [portfolioRes, analyticsRes] = await Promise.all([
            queryInsiderPortfolio(wallet.id),
            queryInsiderAnalytics(wallet.id, '7d'),
          ]);
          balances[wallet.id] = portfolioRes.data.items.reduce(
            (sum, item) => sum + (item.current_value || item.usd_value || 0),
            0
          );
          analytics[wallet.id] = analyticsRes.data.item;
        })
      );
      analyticsMap.value = analytics;
      balanceMap.value = balances;
    } finally {
      loading.value = false;
    }
  };

  const pollSyncStatus = async (attempt = 0): Promise<boolean> => {
    if (attempt >= 30) return false;
    await wait(2000);
    const { data } = await queryInsiderSyncStatus();
    if (!data.item.syncing) {
      if (data.item.last_error) Message.error(data.item.last_error);
      else Message.success(`同步完成，当前引擎：${data.item.engine}`);
      await loadData();
      return true;
    }
    return pollSyncStatus(attempt + 1);
  };

  const openCreate = () => {
    editing.value = null;
    form.address = '';
    form.label = '';
    modalVisible.value = true;
  };

  const openEdit = (wallet: InsiderWallet) => {
    editing.value = wallet;
    form.address = wallet.address;
    form.label = wallet.label;
    modalVisible.value = true;
  };

  const handleSubmit = async () => {
    if (editing.value) {
      await updateInsiderWallet(editing.value.id, form.label);
      Message.success('钱包已更新');
    } else {
      await createInsiderWallet({ address: form.address, label: form.label });
      Message.success('钱包已添加');
    }
    modalVisible.value = false;
    await loadData();
  };

  const handleDelete = async (id: number) => {
    await deleteInsiderWallet(id);
    Message.success('钱包已删除');
    await loadData();
  };

  const handleSync = async () => {
    syncing.value = true;
    try {
      await triggerInsiderSync();
      const completed = await pollSyncStatus();
      if (!completed) Message.info('同步仍在进行中');
    } finally {
      syncing.value = false;
    }
  };

  const goDetail = (id: number) =>
    router.push({ name: 'InsiderWalletDetail', params: { id } });

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

  .wallet-card {
    cursor: pointer;
  }

  .summary-card {
    height: 100%;
  }

  .summary-card :deep(.arco-card-body) {
    padding: 20px;
  }
</style>
