import axios from 'axios';
import { auth } from '../firebase';
import { getIdToken } from 'firebase/auth';

const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use(async (config) => {
  const user = auth.currentUser;
  if (user) {
    const token = await getIdToken(user);
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export const shortenUrl = async (longUrl, customSlug) => {
  const response = await api.post('/shorten', {
    long_url: longUrl,
    custom_slug: customSlug,
  });
  return response.data;
};

export const getTopAnalytics = async (page = 1, limit = 10) => {
  const response = await api.get('/analytics/top', {
    params: { page, limit },
  });
  return response.data;
};

export const getUserHistory = async () => {
  const response = await api.get('/user/history');
  return response.data;
};

export const updateUrl = async (slug, longUrl) => {
  const response = await api.patch(`/links/${slug}`, {
    long_url: longUrl,
  });
  return response.data;
};

export const deleteUrl = async (slug) => {
  console.log("deleteUrl called for slug:", slug);
  const response = await api.delete(`/links/${slug}`);
  return response.data;
};

export default api;
