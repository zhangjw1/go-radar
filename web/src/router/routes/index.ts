import type { RouteRecordNormalized } from 'vue-router';
import dashboard from './modules/dashboard';
import insider from './modules/insider';
import visualization from './modules/visualization';
import list from './modules/list';
import form from './modules/form';
import message from './modules/message';
import profile from './modules/profile';
import user from './modules/user';
import faq from './externalModules/faq';

export const appRoutes = [
  dashboard,
  insider,
  visualization,
  list,
  form,
  message,
  profile,
  user,
] as unknown as RouteRecordNormalized[];

export const appExternalRoutes = [faq] as unknown as RouteRecordNormalized[];
