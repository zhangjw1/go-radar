import { mergeConfig } from 'vite';
import baseConfig from './vite.config.base';
import configArcoResolverPlugin from './plugin/arcoResolver';

export default mergeConfig(
  baseConfig,
  {
    base: '/app/',
    mode: 'production',
    plugins: [configArcoResolverPlugin()],
    build: {
      rollupOptions: {
        output: {
          manualChunks: {
            arco: ['@arco-design/web-vue'],
            chart: ['echarts', 'vue-echarts'],
            vue: ['vue', 'vue-router', 'pinia', '@vueuse/core', 'vue-i18n'],
          },
        },
      },
      chunkSizeWarningLimit: 2000,
    },
  }
);
