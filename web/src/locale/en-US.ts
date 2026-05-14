import localeMessageBox from '@/components/message-box/locale/en-US';
import localeLogin from '@/views/login/locale/en-US';

import localeWorkplace from '@/views/dashboard/workplace/locale/en-US';
/** simple */
import localeMonitor from '@/views/dashboard/monitor/locale/en-US';

import localeSearchTable from '@/views/list/search-table/locale/en-US';
import localeCardList from '@/views/list/card/locale/en-US';

import localeStepForm from '@/views/form/step/locale/en-US';
import localeGroupForm from '@/views/form/group/locale/en-US';

import localeBasicProfile from '@/views/profile/basic/locale/en-US';

import localeDataAnalysis from '@/views/visualization/data-analysis/locale/en-US';
import localeMultiDAnalysis from '@/views/visualization/multi-dimension-data-analysis/locale/en-US';

import localeUserInfo from '@/views/user/info/locale/en-US';
import localeUserSetting from '@/views/user/setting/locale/en-US';
/** simple end */
import localeSettings from './en-US/settings';

export default {
  'menu.dashboard': 'Dashboard',
  'menu.dashboard.overview': 'Overview',
  'menu.dashboard.workplace': 'Workplace',
  'menu.dashboard.monitor': 'Monitor',
  'menu.radar.dashboard': 'Radar Dashboard',
  'menu.radar.signals': 'Signals',
  'menu.radar.s1': 'S1 Binance',
  'menu.radar.s2': 'S2 Funding Flip',
  'menu.radar.s3': 'S3 Heat',
  'menu.radar.s5': 'S5 On-chain',
  'menu.radar.s7': 'S7 Vitalik Sell',
  'menu.radar.jobs': 'Jobs',
  'menu.radar.pushes': 'TG Pushes',
  'menu.radar.settings': 'Settings',
  'menu.insider': 'Wallet Monitor',
  'menu.insider.wallets': 'Wallets',
  'menu.insider.detail': 'Wallet Detail',
  'menu.insider.settings': 'Monitor Settings',
  'menu.server.dashboard': 'Dashboard-Server',
  'menu.server.workplace': 'Workplace-Server',
  'menu.server.monitor': 'Monitor-Server',
  'menu.message': 'Message',
  'menu.list': 'List',
  'menu.list.searchTable': 'Search Table',
  'menu.list.cardList': 'Card List',
  'menu.form': 'Form',
  'menu.form.step': 'Step Form',
  'menu.form.group': 'Group Form',
  'menu.profile': 'Profile',
  'menu.profile.basic': 'Basic Profile',
  'menu.visualization': 'Data Visualization',
  'menu.visualization.dataAnalysis': 'Data Analysis',
  'menu.visualization.multiDimensionDataAnalysis':
    'Multi-Dimension Data Analysis',
  'menu.user': 'User Center',
  'menu.user.info': 'User Info',
  'menu.user.setting': 'User Setting',
  'menu.faq': 'FAQ',
  'navbar.docs': 'Docs',
  'navbar.action.locale': 'Switch to English',
  ...localeSettings,
  ...localeMessageBox,
  ...localeLogin,
  ...localeWorkplace,
  /** simple */
  ...localeMonitor,
  ...localeSearchTable,
  ...localeCardList,
  ...localeStepForm,
  ...localeGroupForm,
  ...localeBasicProfile,
  ...localeDataAnalysis,
  ...localeMultiDAnalysis,
  ...localeUserInfo,
  ...localeUserSetting,
  /** simple end */
};
