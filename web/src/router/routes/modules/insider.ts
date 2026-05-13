import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const INSIDER: AppRouteRecordRaw = {
  path: '/insider',
  name: 'insider',
  component: DEFAULT_LAYOUT,
  meta: {
    locale: 'menu.insider',
    requiresAuth: true,
    icon: 'icon-safe',
    order: 2,
    roles: ['*'],
  },
  children: [
    {
      path: 'wallets',
      name: 'InsiderWallets',
      component: () => import('@/views/insider/wallets/index.vue'),
      meta: {
        locale: 'menu.insider.wallets',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'wallets/:id',
      name: 'InsiderWalletDetail',
      component: () => import('@/views/insider/detail/index.vue'),
      meta: {
        locale: 'menu.insider.detail',
        requiresAuth: true,
        hideInMenu: true,
        roles: ['*'],
      },
    },
    {
      path: 'settings',
      name: 'InsiderSettings',
      component: () => import('@/views/insider/settings/index.vue'),
      meta: {
        locale: 'menu.insider.settings',
        requiresAuth: true,
        roles: ['*'],
      },
    },
  ],
};

export default INSIDER;
