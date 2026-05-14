<template>
  <div class="container">
    <Breadcrumb :items="['menu.insider', 'menu.insider.settings']" />
    <a-space direction="vertical" fill size="large">
      <a-page-header
        title="监控设置"
        subtitle="配置钱包监控引擎、通知渠道和告警规则。"
      >
        <template #extra>
          <a-space>
            <a-select
              v-model="engine"
              style="width: 180px"
              @change="saveEngine"
            >
              <a-option value="service">服务引擎</a-option>
              <a-option value="legacy">兼容模式</a-option>
            </a-select>
            <a-button :loading="syncing" @click="handleSync">执行同步</a-button>
          </a-space>
        </template>
      </a-page-header>

      <a-card class="general-card settings-card" :bordered="false">
        <a-tabs default-active-key="channels" type="rounded">
          <a-tab-pane key="channels" title="通知渠道">
            <a-row justify="space-between" align="center" class="section-title">
              <a-typography-title :heading="6">渠道列表</a-typography-title>
              <a-button type="primary" @click="openChannel()"
                >新增渠道</a-button
              >
            </a-row>
            <a-table
              :data="channels"
              row-key="id"
              :loading="loading"
              :pagination="false"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="名称" data-index="name" />
                <a-table-column title="类型" data-index="channel_type">
                  <template #cell="{ record }"
                    ><a-tag>{{ record.channel_type }}</a-tag></template
                  >
                </a-table-column>
                <a-table-column title="接收对象" data-index="recipient">
                  <template #cell="{ record }">{{
                    record.recipient || '-'
                  }}</template>
                </a-table-column>
                <a-table-column title="启用状态" data-index="enabled">
                  <template #cell="{ record }">{{
                    record.enabled ? '启用' : '停用'
                  }}</template>
                </a-table-column>
                <a-table-column title="操作">
                  <template #cell="{ record }">
                    <a-button size="mini" @click="openChannel(record)"
                      >编辑</a-button
                    >
                  </template>
                </a-table-column>
              </template>
            </a-table>
          </a-tab-pane>

          <a-tab-pane key="rules" title="告警规则">
            <a-row justify="space-between" align="center" class="section-title">
              <a-typography-title :heading="6">规则列表</a-typography-title>
              <a-button type="primary" @click="openRule()">新增规则</a-button>
            </a-row>
            <a-table
              :data="rules"
              row-key="id"
              :loading="loading"
              :pagination="false"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="规则类型" data-index="rule_type" />
                <a-table-column title="阈值" data-index="threshold" />
                <a-table-column title="关联钱包">
                  <template #cell="{ record }">
                    {{
                      record.wallet
                        ? record.wallet.label ||
                          shortAddress(record.wallet.address)
                        : '全局'
                    }}
                  </template>
                </a-table-column>
                <a-table-column title="通知渠道">
                  <template #cell="{ record }">{{
                    channelNames(record.channel_ids)
                  }}</template>
                </a-table-column>
                <a-table-column title="启用状态">
                  <template #cell="{ record }">{{
                    record.enabled ? '启用' : '停用'
                  }}</template>
                </a-table-column>
                <a-table-column title="操作">
                  <template #cell="{ record }">
                    <a-button size="mini" @click="openRule(record)"
                      >编辑</a-button
                    >
                  </template>
                </a-table-column>
              </template>
            </a-table>
          </a-tab-pane>

          <a-tab-pane key="history" title="告警历史">
            <a-table
              :data="history"
              row-key="id"
              :loading="loading"
              :pagination="{ pageSize: 12 }"
              :bordered="false"
            >
              <template #columns>
                <a-table-column title="时间" data-index="created_at">
                  <template #cell="{ record }">{{
                    formatTime(record.created_at)
                  }}</template>
                </a-table-column>
                <a-table-column title="类型" data-index="alert_type" />
                <a-table-column title="级别" data-index="level">
                  <template #cell="{ record }"
                    ><a-tag>{{ record.level }}</a-tag></template
                  >
                </a-table-column>
                <a-table-column title="内容" data-index="message" />
              </template>
            </a-table>
          </a-tab-pane>
        </a-tabs>
      </a-card>
    </a-space>

    <a-modal
      v-model:visible="channelVisible"
      title="通知渠道"
      @ok="saveChannel"
    >
      <a-form :model="channelForm" layout="vertical">
        <a-form-item label="名称"
          ><a-input v-model="channelForm.name"
        /></a-form-item>
        <a-form-item label="类型">
          <a-select v-model="channelForm.channel_type">
            <a-option value="telegram">Telegram</a-option>
            <a-option value="discord">Discord</a-option>
            <a-option value="wechat">企业微信</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="接收对象"
          ><a-input v-model="channelForm.recipient"
        /></a-form-item>
        <a-form-item
          v-if="channelForm.channel_type !== 'telegram'"
          label="Webhook 地址"
        >
          <a-input v-model="channelForm.webhook_url" />
        </a-form-item>
        <template v-if="channelForm.channel_type === 'telegram'">
          <a-form-item label="Bot Token"
            ><a-input v-model="channelForm.bot_token"
          /></a-form-item>
          <a-form-item label="Chat ID"
            ><a-input v-model="channelForm.chat_id"
          /></a-form-item>
        </template>
        <a-form-item label="是否启用"
          ><a-switch v-model="channelForm.enabled"
        /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:visible="ruleVisible" title="告警规则" @ok="saveRule">
      <a-form :model="ruleForm" layout="vertical">
        <a-form-item label="规则类型">
          <a-select v-model="ruleForm.rule_type">
            <a-option value="balance_change">余额变动</a-option>
            <a-option value="new_token">新增代币</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="关联钱包">
          <a-select
            :model-value="ruleForm.wallet_id ?? undefined"
            allow-clear
            placeholder="全局规则"
            @change="setRuleWallet"
          >
            <a-option
              v-for="wallet in wallets"
              :key="wallet.id"
              :value="wallet.id"
            >
              {{ wallet.label || shortAddress(wallet.address) }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="阈值">
          <a-input-number
            v-model="ruleForm.threshold"
            :disabled="ruleForm.rule_type === 'new_token'"
            :min="0"
          />
        </a-form-item>
        <a-form-item label="通知渠道">
          <a-select v-model="ruleForm.channel_ids" multiple>
            <a-option
              v-for="channel in channels"
              :key="channel.id"
              :value="channel.id"
            >
              {{ channel.name }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="是否启用"
          ><a-switch v-model="ruleForm.enabled"
        /></a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script lang="ts" setup>
  import { onMounted, reactive, ref } from 'vue';
  import { Message } from '@arco-design/web-vue';
  import { querySettings } from '@/api/radar';
  import {
    queryAlertHistory,
    queryAlertRules,
    queryInsiderSyncStatus,
    queryInsiderWallets,
    queryNotificationChannels,
    saveAlertRule,
    saveNotificationChannel,
    triggerInsiderSync,
    updateInsiderEngine,
    type AlertHistory,
    type AlertRule,
    type InsiderWallet,
    type NotificationChannel,
  } from '@/api/insider';

  const loading = ref(false);
  const syncing = ref(false);
  const engine = ref<'service' | 'legacy'>('service');
  const wallets = ref<InsiderWallet[]>([]);
  const channels = ref<NotificationChannel[]>([]);
  const rules = ref<AlertRule[]>([]);
  const history = ref<AlertHistory[]>([]);
  const channelVisible = ref(false);
  const ruleVisible = ref(false);

  const channelForm = reactive<NotificationChannel>({
    name: '',
    channel_type: 'telegram',
    recipient: '',
    webhook_url: '',
    bot_token: '',
    chat_id: '',
    enabled: true,
  });

  const ruleForm = reactive<AlertRule>({
    wallet_id: null,
    rule_type: 'balance_change',
    threshold: 0.2,
    channel_ids: [],
    enabled: true,
  });

  const shortAddress = (value: string) =>
    `${value.slice(0, 6)}...${value.slice(-6)}`;
  const formatTime = (value: string) =>
    value ? new Date(value).toLocaleString() : '-';
  const channelNames = (ids: number[] = []) =>
    ids
      .map((id) => channels.value.find((item) => item.id === id)?.name)
      .filter(Boolean)
      .join(', ') || '-';
  const wait = (ms: number) =>
    new Promise((resolve) => {
      setTimeout(resolve, ms);
    });

  const setRuleWallet = (
    value:
      | string
      | number
      | boolean
      | Record<string, any>
      | Array<string | number | boolean | Record<string, any>>
      | undefined
  ) => {
    ruleForm.wallet_id = typeof value === 'number' ? value : null;
  };

  const loadData = async () => {
    loading.value = true;
    try {
      const [walletRes, channelRes, ruleRes, historyRes, settingsRes] =
        await Promise.all([
          queryInsiderWallets(),
          queryNotificationChannels(),
          queryAlertRules(),
          queryAlertHistory(),
          querySettings(),
        ]);
      wallets.value = walletRes.data.items;
      channels.value = channelRes.data.items;
      rules.value = ruleRes.data.items;
      history.value = historyRes.data.items;
      const settings = settingsRes.data.settings as Record<string, string>;
      if (settings.insider_monitor_engine === 'legacy') engine.value = 'legacy';
      else engine.value = 'service';
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

  const saveEngine = async () => {
    await updateInsiderEngine(engine.value);
    Message.success('监控引擎已更新');
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

  const openChannel = (channel?: NotificationChannel) => {
    Object.assign(
      channelForm,
      channel || {
        id: undefined,
        name: '',
        channel_type: 'telegram',
        recipient: '',
        webhook_url: '',
        bot_token: '',
        chat_id: '',
        enabled: true,
      }
    );
    channelVisible.value = true;
  };

  const saveChannel = async () => {
    await saveNotificationChannel(channelForm);
    channelVisible.value = false;
    Message.success('通知渠道已保存');
    await loadData();
  };

  const openRule = (rule?: AlertRule) => {
    Object.assign(
      ruleForm,
      rule || {
        id: undefined,
        wallet_id: null,
        rule_type: 'balance_change',
        threshold: 0.2,
        channel_ids: [],
        enabled: true,
      }
    );
    ruleVisible.value = true;
  };

  const saveRule = async () => {
    if (ruleForm.rule_type === 'new_token') ruleForm.threshold = 0;
    await saveAlertRule(ruleForm);
    ruleVisible.value = false;
    Message.success('告警规则已保存');
    await loadData();
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

  .section-title {
    margin-bottom: 16px;
  }

  .settings-card :deep(.arco-tabs-content) {
    padding-top: 4px;
  }
</style>
