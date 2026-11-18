import { mergeConfig } from 'vite';
import baseConfig from './vite.config.base';

export default mergeConfig(baseConfig, {
  base: '/app/',
  mode: 'production',
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia', '@vueuse/core'],
        },
      },
    },
    chunkSizeWarningLimit: 2000,
  },
});
