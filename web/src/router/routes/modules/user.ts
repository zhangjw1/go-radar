import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const USER: AppRouteRecordRaw = {
  path: '/user',
  name: 'user',
  component: DEFAULT_LAYOUT,
  meta: {
    locale: 'menu.user',
    icon: 'icon-user',
    requiresAuth: true,
    order: 8,
  },
  children: [
    {
      path: 'info',
      name: 'Info',
      component: () => import('@/views/user/info/index.vue'),
      meta: {
        locale: 'menu.user.info',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'setting',
      name: 'Setting',
      component: () => import('@/views/user/setting/index.vue'),
      meta: {
        locale: 'menu.user.setting',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'radar-jobs',
      name: 'RadarJobs',
      alias: '/radar/jobs',
      component: () => import('@/views/radar/jobs/index.vue'),
      meta: { locale: 'menu.radar.jobs', requiresAuth: true, roles: ['*'] },
    },
    {
      path: 'radar-settings',
      name: 'RadarSettings',
      component: () => import('@/views/radar/settings/index.vue'),
      meta: {
        locale: 'menu.radar.settings',
        requiresAuth: true,
        roles: ['*'],
      },
    },
  ],
};

export default USER;
