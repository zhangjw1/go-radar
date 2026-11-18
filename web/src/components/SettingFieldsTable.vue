<template>
  <div class="border border-slate-200 rounded-lg overflow-x-auto">
    <table class="w-full text-left border-collapse text-xs">
      <thead class="bg-slate-50 text-[11px] text-slate-500">
        <tr>
          <th class="py-2 px-3">配置项</th>
          <th class="py-2 px-3">字段 key</th>
          <th class="py-2 px-3">当前值</th>
          <th class="py-2 px-3">默认值</th>
          <th class="py-2 px-3">覆盖状态</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100">
        <tr
          v-for="row in rows"
          :key="row.meta.key"
          class="hover:bg-slate-50/70"
        >
          <td class="py-2.5 px-3">
            <div class="flex items-center gap-1.5">
              <span class="font-semibold text-slate-800">{{
                row.meta.name
              }}</span>
              <span
                v-if="row.meta.envOnly"
                class="px-1.5 py-0.5 rounded bg-amber-50 text-amber-700 border border-amber-200 text-[10px] font-semibold"
              >
                仅环境变量生效
              </span>
            </div>
            <div class="text-[11px] text-slate-400 mt-0.5">
              {{ row.meta.description }}
            </div>
          </td>
          <td class="py-2.5 px-3">
            <span
              class="bg-slate-100 text-slate-700 font-mono text-[11px] px-1.5 py-0.5 rounded"
            >
              {{ row.meta.key }}
            </span>
          </td>
          <td class="py-2.5 px-3">
            <template v-if="row.meta.inputType === 'boolean'">
              <button
                type="button"
                :disabled="row.meta.envOnly"
                class="px-2.5 py-1 rounded text-[11px] font-bold border cursor-pointer disabled:opacity-50"
                :class="
                  settingsStore.draftValues[row.meta.key] === 'true'
                    ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                    : 'bg-slate-100 text-slate-600 border-slate-200'
                "
                @click="toggleBoolean(row.meta.key)"
              >
                {{
                  settingsStore.draftValues[row.meta.key] === 'true'
                    ? '开启'
                    : '关闭'
                }}
              </button>
            </template>
            <template v-else>
              <input
                v-model="settingsStore.draftValues[row.meta.key]"
                :type="row.meta.inputType === 'password' ? 'password' : 'text'"
                :disabled="row.meta.envOnly"
                class="border border-slate-200 rounded px-2 py-1 font-mono text-xs w-40 bg-white focus:outline-none focus:border-blue-500 disabled:bg-slate-100 disabled:text-slate-400"
              />
            </template>
          </td>
          <td class="py-2.5 px-3 font-mono text-slate-500">
            {{ row.defaultValue || '-' }}
          </td>
          <td class="py-2.5 px-3">
            <span
              v-if="row.override"
              class="text-blue-600 font-mono font-semibold"
            >
              已覆盖
            </span>
            <span v-else class="text-slate-300">未覆盖</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
  import { useSettingsStore } from '@/store';
  import type { SettingMetaItem } from '@/constants/radar';

  interface SettingTableRow {
    meta: SettingMetaItem;
    value: string;
    defaultValue: string;
    override: string;
  }

  defineProps<{
    rows: SettingTableRow[];
  }>();

  const settingsStore = useSettingsStore();

  const toggleBoolean = (key: string) => {
    const cur = settingsStore.draftValues[key] === 'true';
    settingsStore.draftValues[key] = cur ? 'false' : 'true';
  };
</script>
