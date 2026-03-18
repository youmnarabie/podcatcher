// web/src/api.ts
import axios from 'axios';
import type { Episode, EpisodeListParams, EpisodeWithFeed, Feed, FeedRule, PlaybackState, Series } from './types';

const client = axios.create({ baseURL: '/api/v1' });

export const api = {
  listFeeds: () => client.get<Feed[]>('/feeds').then(r => r.data),
  createFeed: (url: string) => client.post<Feed>('/feeds', { url }).then(r => r.data),
  deleteFeed: (id: string) => client.delete(`/feeds/${id}`),
  refreshFeed: (id: string) => client.post(`/feeds/${id}/refresh`),

  listEpisodes: (params: EpisodeListParams) =>
    client.get<Episode[]>('/episodes', { params }).then(r => r.data),
  getEpisode: (id: string) => client.get<Episode>(`/episodes/${id}`).then(r => r.data),
  getPlayback: (id: string) => client.get<PlaybackState>(`/episodes/${id}/playback`).then(r => r.data),
  upsertPlayback: (id: string, position_seconds: number, completed: boolean) =>
    client.put<PlaybackState>(`/episodes/${id}/playback`, { position_seconds, completed }).then(r => r.data),
  addEpisodeSeries: (id: string, series_id: string, episode_number?: number) =>
    client.post(`/episodes/${id}/series`, { series_id, episode_number }),
  removeEpisodeSeries: (id: string, seriesId: string) =>
    client.delete(`/episodes/${id}/series/${seriesId}`),

  listSeries: (feedId: string) =>
    client.get<Series[]>(`/feeds/${feedId}/series`).then(r => r.data),
  createSeries: (feedId: string, name: string) =>
    client.post<Series>(`/feeds/${feedId}/series`, { name }).then(r => r.data),
  renameSeries: (id: string, name: string) => client.patch(`/series/${id}`, { name }),

  listRules: (feedId: string) =>
    client.get<FeedRule[]>(`/feeds/${feedId}/rules`).then(r => r.data),
  createRule: (feedId: string, pattern: string, priority: number) =>
    client.post<FeedRule>(`/feeds/${feedId}/rules`, { pattern, priority }).then(r => r.data),
  updateRule: (id: string, pattern: string, priority: number) =>
    client.patch(`/rules/${id}`, { pattern, priority }),
  deleteRule: (id: string) => client.delete(`/rules/${id}`),

  exportOPML: () => client.get('/opml/export', { responseType: 'blob' }),
  search: (q: string) =>
    client.get<{ Episodes: EpisodeWithFeed[]; Feeds: Feed[] }>(`/search?q=${encodeURIComponent(q)}`).then(r => r.data),
};
