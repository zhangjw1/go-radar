import axios from 'axios';
import type { AxiosRequestConfig, AxiosResponse } from 'axios';

export interface HttpResponse<T = unknown> {
  status: number;
  msg: string;
  code: number;
  data: T;
}

if (import.meta.env.VITE_API_BASE_URL) {
  axios.defaults.baseURL = import.meta.env.VITE_API_BASE_URL;
}

axios.interceptors.request.use(
  (config: AxiosRequestConfig) => config,
  (error) => Promise.reject(error)
);

axios.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error) => {
    const message =
      error?.response?.data?.error ||
      error?.response?.data?.message ||
      error?.message ||
      'Request Error';
    console.error('[api] request failed:', message);
    return Promise.reject(error);
  }
);
