// web/src/types.ts
export interface Feed {
  ID: string; URL: string; Title: string | null; Description: string | null;
  ImageURL: string | null; PollIntervalSeconds: number;
  LastFetchedAt: string | null; CreatedAt: string;
}
export interface Episode {
  ID: string; FeedID: string; GUID: string; Title: string;
  Description: string | null; AudioURL: string; DurationSeconds: number | null;
  PublishedAt: string | null; RawSeason: string | null;
  RawEpisodeNumber: string | null; CreatedAt: string;
}
export interface Series { ID: string; FeedID: string; Name: string; CreatedAt: string; }
export interface PlaybackState {
  ID: string; EpisodeID: string; PositionSeconds: number;
  Completed: boolean; UpdatedAt: string;
}
export interface FeedRule {
  ID: string; FeedID: string; Pattern: string; Priority: number; CreatedAt: string;
}
export interface EpisodeListParams {
  feed_id?: string; series_id?: string; played?: boolean;
  sort?: 'published_at' | 'duration' | 'title';
  order?: 'asc' | 'desc';
  date_from?: string; date_to?: string;
  limit?: number; offset?: number;
}
export interface EpisodeWithFeed extends Episode {
  FeedTitle: string;
}
