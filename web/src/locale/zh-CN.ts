import localeMessageBox from '@/components/message-box/locale/zh-CN';
import localeLogin from '@/views/login/locale/zh-CN';

import localeWorkplace from '@/views/dashboard/workplace/locale/zh-CN';
/** simple */
import localeMonitor from '@/views/dashboard/monitor/locale/zh-CN';

import localeSearchTable from '@/views/list/search-table/locale/zh-CN';
import localeCardList from '@/views/list/card/locale/zh-CN';

import localeStepForm from '@/views/form/step/locale/zh-CN';
import localeGroupForm from '@/views/form/group/locale/zh-CN';

import localeBasicProfile from '@/views/profile/basic/locale/zh-CN';

import localeDataAnalysis from '@/views/visualization/data-analysis/locale/zh-CN';
import localeMultiDAnalysis from '@/views/visualization/multi-dimension-data-analysis/locale/zh-CN';

import localeUserInfo from '@/views/user/info/locale/zh-CN';
import localeUserSetting from '@/views/user/setting/locale/zh-CN';
/** simple end */
import localeSettings from './zh-CN/settings';

export default {
  'menu.dashboard': '仪表盘',
  'menu.dashboard.overview': '总览',
  'menu.dashboard.workplace': '工作台',
  'menu.dashboard.monitor': '实时监控',
  'menu.radar.dashboard': '雷达总览',
  'menu.radar.signals': '信号列表',
  'menu.radar.s1': 'S1 币安公告',
  'menu.radar.s2': 'S2 费率反转',
  'menu.radar.s3': 'S3 热度确认',
  'menu.radar.s5': 'S5 链上发现',
  'menu.radar.s7': 'S7 Vitalik Sell',
  'menu.radar.jobs': '任务状态',
  'menu.radar.pushes': 'TG 推送',
  'menu.radar.settings': '运行设置',
  'menu.insider': '钱包监控',
  'menu.insider.wallets': '钱包列表',
  'menu.insider.detail': '钱包详情',
  'menu.insider.settings': '监控设置',
  'menu.server.dashboard': '仪表盘-服务端',
  'menu.server.workplace': '工作台-服务端',
  'menu.server.monitor': '实时监控-服务端',
  'menu.message': '消息管理',
  'menu.list': '列表页',
  'menu.list.searchTable': '查询表格',
  'menu.list.cardList': '卡片列表',
  'menu.form': '表单页',
  'menu.form.step': '分步表单',
  'menu.form.group': '分组表单',
  'menu.profile': '详情页',
  'menu.profile.basic': '基础详情页',
  'menu.visualization': '数据可视化',
  'menu.visualization.dataAnalysis': '分析页',
  'menu.visualization.multiDimensionDataAnalysis': '多维数据分析',
  'menu.user': '个人中心',
  'menu.user.info': '用户信息',
  'menu.user.setting': '用户设置',
  'menu.faq': '常见问题',
  'navbar.docs': '文档中心',
  'navbar.action.locale': '切换为中文',
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
