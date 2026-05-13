import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const MESSAGE: AppRouteRecordRaw = {
  path: '/message',
  name: 'message',
  component: DEFAULT_LAYOUT,
  meta: {
    locale: 'menu.message',
    requiresAuth: true,
    icon: 'icon-notification',
    order: 4,
    roles: ['*'],
  },
  children: [
    {
      path: 'tg-pushes',
      name: 'RadarPushes',
      component: () => import('@/views/radar/pushes/index.vue'),
      meta: {
        locale: 'menu.radar.pushes',
        requiresAuth: true,
        roles: ['*'],
      },
    },
  ],
};

export default MESSAGE;
