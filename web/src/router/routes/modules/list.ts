import { DEFAULT_LAYOUT } from '../base';
import { AppRouteRecordRaw } from '../types';

const LIST: AppRouteRecordRaw = {
  path: '/list',
  name: 'list',
  component: DEFAULT_LAYOUT,
  meta: {
    locale: 'menu.list',
    requiresAuth: true,
    icon: 'icon-list',
    order: 2,
    roles: ['*'],
  },
  children: [
    {
      path: 'search-table',
      name: 'SearchTable',
      component: () => import('@/views/list/search-table/index.vue'),
      meta: {
        locale: 'menu.list.searchTable',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'card',
      name: 'Card',
      component: () => import('@/views/list/card/index.vue'),
      meta: {
        locale: 'menu.list.cardList',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 'signals',
      name: 'RadarSignals',
      alias: '/radar/signals',
      component: () => import('@/views/radar/signals/index.vue'),
      meta: {
        locale: 'menu.radar.signals',
        requiresAuth: true,
        roles: ['*'],
      },
    },
    {
      path: 's1',
      name: 'RadarS1',
      component: () => import('@/views/radar/source/index.vue'),
      meta: { locale: 'menu.radar.s1', requiresAuth: true, roles: ['*'] },
      props: { source: 's1' },
    },
    {
      path: 's2',
      name: 'RadarS2',
      component: () => import('@/views/radar/source/index.vue'),
      meta: { locale: 'menu.radar.s2', requiresAuth: true, roles: ['*'] },
      props: { source: 's2' },
    },
    {
      path: 's3',
      name: 'RadarS3',
      component: () => import('@/views/radar/source/index.vue'),
      meta: { locale: 'menu.radar.s3', requiresAuth: true, roles: ['*'] },
      props: { source: 's3' },
    },
    {
      path: 's5',
      name: 'RadarS5',
      component: () => import('@/views/radar/source/index.vue'),
      meta: { locale: 'menu.radar.s5', requiresAuth: true, roles: ['*'] },
      props: { source: 's5' },
    },
    {
      path: 's7',
      name: 'RadarS7',
      component: () => import('@/views/radar/source/index.vue'),
      meta: { locale: 'menu.radar.s7', requiresAuth: true, roles: ['*'] },
      props: { source: 's7' },
    },
    {
      path: 'token/:chain/:address',
      name: 'RadarTokenDetail',
      component: () => import('@/views/radar/token/index.vue'),
      meta: {
        locale: 'menu.profile.basic',
        requiresAuth: true,
        hideInMenu: true,
        activeMenu: 'RadarSignals',
        roles: ['*'],
      },
    },
  ],
};

export default LIST;
