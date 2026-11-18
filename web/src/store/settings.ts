import { defineStore } from 'pinia';
import {
  querySettings,
  sendTelegramTest,
  updateSettings,
  type SettingItem,
} from '@/api/radar';

export interface NormalizedSettingRow {
  key: string;
  value: string;
  defaultValue: string;
  override: string;
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    loading: false,
    saving: false,
    rows: [] as NormalizedSettingRow[],
    draftValues: {} as Record<string, string>,
  }),
  actions: {
    async fetchSettings() {
      this.loading = true;
      try {
        const { data } = await querySettings();
        const payload = data.settings || [];
        const overrides = data.overrides || {};
        const list: NormalizedSettingRow[] = Array.isArray(payload)
          ? payload.map((item: SettingItem) => ({
              key: item.key,
              value: String(item.value ?? ''),
              defaultValue: String(item.default ?? item.value ?? ''),
              override: Object.prototype.hasOwnProperty.call(
                overrides,
                item.key
              )
                ? String(overrides[item.key] ?? '')
                : '',
            }))
          : Object.entries(payload).map(([key, val]) => ({
              key,
              value: String(val ?? ''),
              defaultValue: String(val ?? ''),
              override: Object.prototype.hasOwnProperty.call(overrides, key)
                ? String(overrides[key] ?? '')
                : '',
            }));
        this.rows = list;
        const draft: Record<string, string> = {};
        list.forEach((row) => {
          draft[row.key] = row.value;
        });
        this.draftValues = draft;
      } finally {
        this.loading = false;
      }
    },
    async saveCategorySettings(keys: string[]) {
      this.saving = true;
      try {
        const payload: Record<string, string> = {};
        keys.forEach((k) => {
          if (Object.prototype.hasOwnProperty.call(this.draftValues, k)) {
            payload[k] = String(this.draftValues[k]);
          }
        });
        await updateSettings(payload);
        await this.fetchSettings();
      } finally {
        this.saving = false;
      }
    },
    async triggerTelegramTest() {
      await sendTelegramTest();
    },
  },
});
