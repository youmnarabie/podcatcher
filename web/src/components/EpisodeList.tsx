// web/src/components/EpisodeList.tsx
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { api } from '../api';
import type { Episode, EpisodeListParams } from '../types';
import EpisodeItem from './EpisodeItem';
import EpisodeFilters from './EpisodeFilters';

interface Props { onPlay?: (ep: Episode) => void; played?: boolean; }

export default function EpisodeList({ onPlay, played }: Props) {
  const { feedId, seriesId } = useParams();
  const [filterParams, setFilterParams] = useState<EpisodeListParams>({
    sort: 'published_at', order: 'desc', played,
  });

  const params: EpisodeListParams = { ...filterParams, feed_id: feedId, series_id: seriesId };
  const { data: episodes = [], isLoading } = useQuery({
    queryKey: ['episodes', params],
    queryFn: () => api.listEpisodes(params),
  });

  if (isLoading) return <p>Loading…</p>;
  return (
    <div>
      <EpisodeFilters params={filterParams} onChange={setFilterParams} />
      {episodes.length === 0 && <p>No episodes.</p>}
      {episodes.map(ep => <EpisodeItem key={ep.ID} episode={ep} onPlay={onPlay} />)}
    </div>
  );
}
