<template>
  <div
    v-if="appStore.settingsOpen"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
    @click.self="handleClose()"
  >
    <div
      class="bg-white rounded-xl shadow-2xl border border-slate-200 max-w-4xl w-full overflow-hidden flex flex-col max-h-[88vh]"
    >
      <!-- 弹窗头部 -->
      <div
        class="p-4 border-b border-slate-100 flex items-center justify-between bg-slate-50/70 shrink-0"
      >
        <div class="flex items-center gap-2">
          <span class="font-bold text-slate-900 text-sm">系统设置</span>
          <span
            class="bg-blue-50 text-blue-700 border border-blue-200 text-[10px] font-mono font-semibold px-2 py-0.5 rounded"
          >
            /radar-api/settings & /jobs
          </span>
        </div>
        <button
          type="button"
          class="w-7 h-7 rounded-md flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-200/50 cursor-pointer text-sm font-semibold"
          @click="handleClose()"
        >
          ✕
        </button>
      </div>

      <!-- 6 个页签 -->
      <div
        class="px-4 border-b border-slate-200 bg-white flex items-center gap-4 text-xs font-semibold shrink-0 overflow-x-auto"
      >
        <button
          v-for="tab in SETTING_TABS"
          :key="tab.key"
          type="button"
          class="py-2.5 border-b-2 cursor-pointer whitespace-nowrap transition-colors"
          :class="
            appStore.settingsTab === tab.key
              ? 'border-slate-900 text-slate-900'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          "
          @click="switchTab(tab.key)"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- 主内容区 -->
      <div class="p-4 overflow-y-auto flex-1 text-xs space-y-4">
        <!-- 1. 任务记录 -->
        <template v-if="appStore.settingsTab === 'jobs'">
          <div class="flex items-center justify-between">
            <span class="text-slate-500"
              >最近 30 条扫描任务运行记录（含 metadata_json 解析）</span
            >
            <button
              type="button"
              class="px-3 py-1 bg-white border border-slate-200 hover:bg-slate-50 rounded-md font-medium cursor-pointer"
              @click="radarStore.fetchJobs(30)"
            >
              刷新记录
            </button>
          </div>
          <div class="border border-slate-200 rounded-lg overflow-x-auto">
            <table class="w-full text-left border-collapse">
              <thead class="bg-slate-50 text-[11px] text-slate-500">
                <tr>
                  <th class="py-2 px-3">开始时间</th>
                  <th class="py-2 px-3">扫描器</th>
                  <th class="py-2 px-3">状态</th>
                  <th class="py-2 px-3 text-right">信号</th>
                  <th class="py-2 px-3 text-right">快照</th>
                  <th class="py-2 px-3">推送 / 共振</th>
                  <th class="py-2 px-3">错误 / 警告</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100">
                <tr
                  v-for="job in parsedJobs"
                  :key="job.id || job.started_at + job.scanner"
                  class="hover:bg-slate-50"
                >
                  <td class="py-2 px-3 font-mono text-slate-500">
                    {{ formatTime(job.started_at) }}
                  </td>
                  <td class="py-2 px-3 font-semibold text-slate-800">
                    {{ sourceLabel(job.scanner) }}
                  </td>
                  <td class="py-2 px-3">
                    <span
                      class="font-bold uppercase text-[10px]"
                      :class="
                        job.status === 'ok'
                          ? 'text-emerald-600'
                          : job.status === 'warning'
                          ? 'text-amber-600'
                          : 'text-rose-600'
                      "
                    >
                      ● {{ job.status }}
                    </span>
                  </td>
                  <td class="py-2 px-3 text-right font-mono font-bold">
                    {{ job.signal_count }}
                  </td>
                  <td class="py-2 px-3 text-right font-mono">
                    {{ job.snapshot_count }}
                  </td>
                  <td class="py-2 px-3 font-mono text-slate-600">
                    {{ job.pushedCount }} / {{ job.resonanceCount }}
                  </td>
                  <td class="py-2 px-3 text-slate-500 max-w-xs truncate">
                    {{ job.error || job.warningsText || '-' }}
                  </td>
                </tr>
                <tr v-if="parsedJobs.length === 0">
                  <td colspan="7" class="py-8 text-center text-slate-400">
                    暂无任务记录
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>

        <!-- 2. 扫描器 -->
        <template v-else-if="appStore.settingsTab === 'scanner'">
          <div
            class="flex items-center justify-between bg-slate-50 p-3 rounded-lg border border-slate-200"
          >
            <div>
              <div class="font-bold text-slate-900">
                后端调度器总开关 (GO_RADAR_ENABLE_SCHEDULER)
              </div>
              <div class="text-[11px] text-slate-500">
                只读展示 /health 返回的
                scheduler_enabled，需在服务端环境变量调整
              </div>
            </div>
            <span
              class="px-2.5 py-1 rounded-md font-bold text-[11px]"
              :class="
                appStore.schedulerEnabled
                  ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                  : 'bg-slate-200 text-slate-600'
              "
            >
              {{
                appStore.schedulerEnabled ? '已启用 (只读)' : '未启用 (只读)'
              }}
            </span>
          </div>
          <SettingFieldsTable :rows="categoryRows('scanner')" />
        </template>

        <!-- 3. 策略参数 -->
        <template v-else-if="appStore.settingsTab === 'strategy'">
          <div
            class="bg-amber-50 border border-amber-200 text-amber-800 rounded-lg p-3 text-[11px]"
          >
            提示：标注「仅环境变量生效」的参数（S3 阈值与 S7
            最小通知金额）目前由后端直接读取环境变量，在线修改不会覆盖运行时行为。
          </div>
          <SettingFieldsTable :rows="categoryRows('strategy')" />
        </template>

        <!-- 4. 推送与 Telegram -->
        <template v-else-if="appStore.settingsTab === 'push'">
          <div class="flex items-center justify-between">
            <span class="font-bold text-slate-800"
              >Telegram 通道与冷却策略</span
            >
            <button
              type="button"
              class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-md font-semibold cursor-pointer"
              @click="handleTestTelegram()"
            >
              发送测试消息
            </button>
          </div>
          <SettingFieldsTable :rows="categoryRows('push')" />

          <div class="pt-2">
            <div class="font-bold text-slate-800 mb-2">
              推送规则说明（只读）
            </div>
            <div class="border border-slate-200 rounded-lg overflow-hidden">
              <table class="w-full text-left border-collapse">
                <thead class="bg-slate-50 text-[11px] text-slate-500">
                  <tr>
                    <th class="py-2 px-3">来源</th>
                    <th class="py-2 px-3">立即推送</th>
                    <th class="py-2 px-3">进入摘要</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100">
                  <tr v-for="rule in PUSH_RULES_DOC" :key="rule.source">
                    <td class="py-2 px-3 font-semibold text-slate-800">
                      {{ rule.source }}
                    </td>
                    <td class="py-2 px-3 text-slate-600">
                      {{ rule.immediate }}
                    </td>
                    <td class="py-2 px-3 text-slate-500">{{ rule.digest }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </template>

        <!-- 5. 网络与数据源 -->
        <template v-else-if="appStore.settingsTab === 'network'">
          <SettingFieldsTable :rows="categoryRows('network')" />
        </template>

        <!-- 6. 钱包告警 -->
        <template v-else-if="appStore.settingsTab === 'alerts'">
          <div
            class="bg-amber-50 border border-amber-200 text-amber-800 rounded-lg p-3 text-[11px]"
          >
            注意：当前钱包告警仅写入告警历史表（t_insider_alert_history），不会实际向外部渠道发送消息。
          </div>

          <div
            class="flex items-center justify-between bg-slate-50 p-3 rounded-lg border border-slate-200"
          >
            <div>
              <div class="font-bold text-slate-900">
                钱包监控引擎 (insider_monitor_engine)
              </div>
              <div class="text-[11px] text-slate-500">
                service：完整持仓 + 交易 + 盈亏；legacy：兼容模式
              </div>
            </div>
            <select
              :value="insiderStore.syncStatus.engine || 'service'"
              class="border border-slate-200 rounded-md px-3 py-1.5 bg-white font-semibold"
              @change="onChangeEngine($event)"
            >
              <option value="service">服务引擎 (service)</option>
              <option value="legacy">兼容模式 (legacy)</option>
            </select>
          </div>

          <!-- 告警规则列表 -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="font-bold text-slate-800">告警规则列表</span>
              <button
                type="button"
                class="px-2.5 py-1 bg-[#09182b] text-white rounded-md font-semibold cursor-pointer"
                @click="openRuleForm()"
              >
                + 新增规则
              </button>
            </div>
            <div
              v-if="ruleFormVisible"
              class="p-3 bg-slate-50 border border-slate-200 rounded-lg grid grid-cols-1 sm:grid-cols-4 gap-2"
            >
              <select
                v-model="ruleDraft.rule_type"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white"
              >
                <option value="balance_change"
                  >balance_change (余额变动)</option
                >
                <option value="new_token">new_token (新增代币)</option>
              </select>
              <input
                v-model.number="ruleDraft.threshold"
                type="number"
                step="0.05"
                :disabled="ruleDraft.rule_type === 'new_token'"
                placeholder="阈值 (如 0.2)"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white font-mono"
              />
              <select
                v-model="ruleDraft.wallet_id"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white"
              >
                <option :value="null">全局钱包</option>
                <option
                  v-for="w in insiderStore.wallets"
                  :key="w.id"
                  :value="w.id"
                >
                  {{ w.label || w.address.slice(0, 6) }}
                </option>
              </select>
              <div class="flex gap-1.5 justify-end">
                <button
                  type="button"
                  class="px-2.5 py-1 border border-slate-200 bg-white rounded cursor-pointer"
                  @click="ruleFormVisible = false"
                >
                  取消
                </button>
                <button
                  type="button"
                  class="px-3 py-1 bg-blue-600 text-white rounded font-semibold cursor-pointer"
                  @click="submitRule()"
                >
                  保存规则
                </button>
              </div>
            </div>
            <div class="border border-slate-200 rounded-lg overflow-hidden">
              <table class="w-full text-left border-collapse">
                <thead class="bg-slate-50 text-[11px] text-slate-500">
                  <tr>
                    <th class="py-2 px-3">规则类型</th>
                    <th class="py-2 px-3">关联钱包</th>
                    <th class="py-2 px-3">阈值</th>
                    <th class="py-2 px-3">状态</th>
                    <th class="py-2 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100">
                  <tr v-for="r in insiderStore.rules" :key="r.id">
                    <td class="py-2 px-3 font-mono font-semibold">
                      {{ r.rule_type }}
                    </td>
                    <td class="py-2 px-3 text-slate-600">
                      {{
                        r.wallet
                          ? r.wallet.label || shortAddress(r.wallet.address)
                          : '全局'
                      }}
                    </td>
                    <td class="py-2 px-3 font-mono">{{ r.threshold }}</td>
                    <td class="py-2 px-3">
                      <span
                        class="font-semibold"
                        :class="
                          r.enabled ? 'text-emerald-600' : 'text-slate-400'
                        "
                      >
                        {{ r.enabled ? '启用' : '停用' }}
                      </span>
                    </td>
                    <td class="py-2 px-3 text-right">
                      <button
                        type="button"
                        class="text-blue-600 hover:underline cursor-pointer"
                        @click="openRuleForm(r)"
                      >
                        编辑
                      </button>
                    </td>
                  </tr>
                  <tr v-if="insiderStore.rules.length === 0">
                    <td colspan="5" class="py-6 text-center text-slate-400">
                      暂无告警规则
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- 通知渠道列表 -->
          <div class="space-y-2 pt-2">
            <div class="flex items-center justify-between">
              <span class="font-bold text-slate-800">通知渠道列表</span>
              <button
                type="button"
                class="px-2.5 py-1 bg-[#09182b] text-white rounded-md font-semibold cursor-pointer"
                @click="openChannelForm()"
              >
                + 新增渠道
              </button>
            </div>
            <div
              v-if="channelFormVisible"
              class="p-3 bg-slate-50 border border-slate-200 rounded-lg grid grid-cols-1 sm:grid-cols-3 gap-2"
            >
              <input
                v-model="channelDraft.name"
                placeholder="渠道名称"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white"
              />
              <select
                v-model="channelDraft.channel_type"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white"
              >
                <option value="telegram">telegram</option>
                <option value="discord">discord</option>
                <option value="wechat">wechat</option>
              </select>
              <input
                v-model="channelDraft.recipient"
                placeholder="接收对象 / 描述"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white"
              />
              <input
                v-if="channelDraft.channel_type === 'telegram'"
                v-model="channelDraft.bot_token"
                type="password"
                placeholder="Bot Token"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white font-mono"
              />
              <input
                v-if="channelDraft.channel_type === 'telegram'"
                v-model="channelDraft.chat_id"
                placeholder="Chat ID"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white font-mono"
              />
              <input
                v-if="channelDraft.channel_type !== 'telegram'"
                v-model="channelDraft.webhook_url"
                placeholder="Webhook URL"
                class="border border-slate-200 rounded px-2 py-1.5 bg-white font-mono sm:col-span-2"
              />
              <div class="flex gap-1.5 justify-end sm:col-span-3">
                <button
                  type="button"
                  class="px-2.5 py-1 border border-slate-200 bg-white rounded cursor-pointer"
                  @click="channelFormVisible = false"
                >
                  取消
                </button>
                <button
                  type="button"
                  class="px-3 py-1 bg-blue-600 text-white rounded font-semibold cursor-pointer"
                  @click="submitChannel()"
                >
                  保存渠道
                </button>
              </div>
            </div>
            <div class="border border-slate-200 rounded-lg overflow-hidden">
              <table class="w-full text-left border-collapse">
                <thead class="bg-slate-50 text-[11px] text-slate-500">
                  <tr>
                    <th class="py-2 px-3">名称</th>
                    <th class="py-2 px-3">类型</th>
                    <th class="py-2 px-3">接收对象</th>
                    <th class="py-2 px-3">状态</th>
                    <th class="py-2 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100">
                  <tr v-for="ch in insiderStore.channels" :key="ch.id">
                    <td class="py-2 px-3 font-semibold">{{ ch.name }}</td>
                    <td class="py-2 px-3 font-mono">{{ ch.channel_type }}</td>
                    <td class="py-2 px-3 text-slate-600">
                      {{ ch.recipient || ch.chat_id || '-' }}
                    </td>
                    <td class="py-2 px-3">
                      <span
                        class="font-semibold"
                        :class="
                          ch.enabled ? 'text-emerald-600' : 'text-slate-400'
                        "
                      >
                        {{ ch.enabled ? '启用' : '停用' }}
                      </span>
                    </td>
                    <td class="py-2 px-3 text-right">
                      <button
                        type="button"
                        class="text-blue-600 hover:underline cursor-pointer"
                        @click="openChannelForm(ch)"
                      >
                        编辑
                      </button>
                    </td>
                  </tr>
                  <tr v-if="insiderStore.channels.length === 0">
                    <td colspan="5" class="py-6 text-center text-slate-400">
                      暂无通知渠道
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </template>
      </div>

      <!-- 底部操作栏 -->
      <div
        class="p-3 bg-slate-50 border-t border-slate-100 flex items-center justify-between shrink-0 text-xs"
      >
        <span class="text-slate-400">修改将作为覆盖值保存到 Go 后端设置表</span>
        <div class="flex gap-2">
          <button
            type="button"
            class="px-3 py-1.5 border border-slate-200 bg-white hover:bg-slate-50 text-slate-600 rounded-md cursor-pointer"
            @click="handleClose()"
          >
            关闭
          </button>
          <button
            v-if="isSaveableCategory"
            type="button"
            :disabled="settingsStore.saving"
            class="px-4 py-1.5 bg-[#0c1a30] hover:bg-[#152a4a] text-white font-semibold rounded-md cursor-pointer disabled:opacity-50"
            @click="saveCurrentCategory()"
          >
            {{ settingsStore.saving ? '保存中...' : '保存设置' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { computed, reactive, ref, watch } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import {
    useAppStore,
    useInsiderStore,
    useRadarStore,
    useSettingsStore,
  } from '@/store';
  import {
    PUSH_RULES_DOC,
    SETTING_META,
    SETTING_TABS,
    type SettingTabKey,
  } from '@/constants/radar';
  import {
    formatTime,
    parseJsonObject,
    shortAddress,
    sourceLabel,
  } from '@/utils/format';
  import type { AlertRule, NotificationChannel } from '@/api/insider';
  import SettingFieldsTable from './SettingFieldsTable.vue';

  const route = useRoute();
  const router = useRouter();
  const appStore = useAppStore();
  const radarStore = useRadarStore();
  const insiderStore = useInsiderStore();
  const settingsStore = useSettingsStore();

  const ruleFormVisible = ref(false);
  const channelFormVisible = ref(false);

  const ruleDraft = reactive<AlertRule>({
    wallet_id: null,
    rule_type: 'balance_change',
    threshold: 0.2,
    channel_ids: [],
    enabled: true,
  });

  const channelDraft = reactive<NotificationChannel>({
    name: '',
    channel_type: 'telegram',
    recipient: '',
    webhook_url: '',
    bot_token: '',
    chat_id: '',
    enabled: true,
  });

  const parsedJobs = computed(() =>
    radarStore.jobs.map((job) => {
      const meta = parseJsonObject(job.metadata_json);
      const warnings = Array.isArray(meta.warnings) ? meta.warnings : [];
      return {
        ...job,
        pushedCount: Number(meta.pushed ?? meta.pushed_count ?? 0),
        resonanceCount: Number(meta.resonance ?? meta.resonance_count ?? 0),
        warningsText: warnings.length ? warnings.join('; ') : '',
      };
    })
  );

  const categoryRows = (cat: 'scanner' | 'strategy' | 'push' | 'network') => {
    const metaList = Object.values(SETTING_META).filter(
      (item) => item.category === cat
    );
    const rowMap = new Map(settingsStore.rows.map((r) => [r.key, r]));
    return metaList.map((meta) => {
      const remote = rowMap.get(meta.key);
      return {
        meta,
        value: settingsStore.draftValues[meta.key] ?? remote?.value ?? '',
        defaultValue: remote?.defaultValue ?? '-',
        override: remote?.override ?? '',
      };
    });
  };

  const isSaveableCategory = computed(() =>
    ['scanner', 'strategy', 'push', 'network'].includes(appStore.settingsTab)
  );

  const switchTab = (tab: SettingTabKey) => {
    appStore.settingsTab = tab;
    if (route.path.startsWith('/signals')) {
      router.replace({
        path: '/signals',
        query: { ...route.query, settings: tab },
      });
    }
  };

  const handleClose = () => {
    appStore.closeSettings();
    if (route.query.settings) {
      const nextQuery = { ...route.query };
      delete nextQuery.settings;
      router.replace({ path: route.path, query: nextQuery });
    }
  };

  const saveCurrentCategory = async () => {
    const cat = appStore.settingsTab;
    if (!['scanner', 'strategy', 'push', 'network'].includes(cat)) return;
    const keys = Object.values(SETTING_META)
      .filter((m) => m.category === cat && !m.envOnly)
      .map((m) => m.key);
    try {
      await settingsStore.saveCategorySettings(keys);
      appStore.showToast('配置已保存至后端', 'success');
    } catch {
      appStore.showToast('保存设置失败', 'error');
    }
  };

  const handleTestTelegram = async () => {
    try {
      await settingsStore.triggerTelegramTest();
      appStore.showToast('Telegram 测试消息已发送', 'success');
    } catch {
      appStore.showToast('Telegram 测试发送失败，请检查配置', 'error');
    }
  };

  const onChangeEngine = async (event: Event) => {
    const value = (event.target as HTMLSelectElement).value as
      | 'service'
      | 'legacy';
    try {
      await insiderStore.changeEngine(value);
      appStore.showToast(`监控引擎已切换为 ${value}`, 'success');
    } catch {
      appStore.showToast('切换监控引擎失败', 'error');
    }
  };

  const openRuleForm = (rule?: AlertRule) => {
    Object.assign(
      ruleDraft,
      rule || {
        id: undefined,
        wallet_id: null,
        rule_type: 'balance_change',
        threshold: 0.2,
        channel_ids: [],
        enabled: true,
      }
    );
    ruleFormVisible.value = true;
  };

  const submitRule = async () => {
    try {
      const payload = { ...ruleDraft };
      if (payload.rule_type === 'new_token') payload.threshold = 0;
      await insiderStore.saveRule(payload);
      ruleFormVisible.value = false;
      appStore.showToast('告警规则已保存', 'success');
    } catch {
      appStore.showToast('保存告警规则失败', 'error');
    }
  };

  const openChannelForm = (ch?: NotificationChannel) => {
    Object.assign(
      channelDraft,
      ch || {
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
    channelFormVisible.value = true;
  };

  const submitChannel = async () => {
    try {
      await insiderStore.saveChannel({ ...channelDraft });
      channelFormVisible.value = false;
      appStore.showToast('通知渠道已保存', 'success');
    } catch {
      appStore.showToast('保存通知渠道失败', 'error');
    }
  };

  watch(
    () => appStore.settingsOpen,
    (open) => {
      if (open) {
        settingsStore.fetchSettings().catch(() => undefined);
        radarStore.fetchJobs(30).catch(() => undefined);
        insiderStore.fetchAlertData().catch(() => undefined);
      }
    }
  );
</script>
