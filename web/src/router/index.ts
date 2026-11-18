import { createRouter, createWebHashHistory } from 'vue-router';

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: '/signals',
    },
    {
      path: '/signals',
      name: 'Signals',
      component: () => import('@/views/signals/SignalsView.vue'),
    },
    {
      path: '/wallets',
      name: 'Wallets',
      component: () => import('@/views/wallets/WalletsView.vue'),
    },
    {
      path: '/wallets/:id',
      name: 'WalletDetail',
      component: () => import('@/views/wallets/WalletsView.vue'),
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/signals',
    },
  ],
  scrollBehavior() {
    return { top: 0 };
  },
});

export default router;
