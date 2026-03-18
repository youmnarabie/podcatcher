import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import type { Episode } from '../types';
import EpisodeItem from './EpisodeItem';

interface Props { onPlay: (ep: Episode) => void; }

export default function SearchResults({ onPlay }: Props) {
  const [searchParams] = useSearchParams();
  const q = searchParams.get('q') ?? '';

  const { data, isLoading, isError } = useQuery({
    queryKey: ['search', q],
    queryFn: () => api.search(q),
    enabled: !!q,
  });

  if (!q) return <p>Type to search</p>;
  if (isLoading) return <p>Searching…</p>;
  if (isError) return <p>Search failed. Please try again.</p>;

  const episodes = data?.Episodes ?? [];
  const feeds = data?.Feeds ?? [];
  const noResults = episodes.length === 0 && feeds.length === 0;

  return (
    <div>
      {noResults && <p>No results for &ldquo;{q}&rdquo;</p>}

      {episodes.length > 0 && (
        <section>
          <h3>Episodes</h3>
          {episodes.map(ep => (
            <div key={ep.ID}>
              <small style={{ color: '#888' }}>{ep.FeedTitle}</small>
              <EpisodeItem episode={ep} onPlay={onPlay} />
            </div>
          ))}
        </section>
      )}

      {feeds.length > 0 && (
        <section>
          <h3>Shows</h3>
          {feeds.map(f => (
            <div key={f.ID} style={{ borderBottom: '1px solid #eee', padding: '8px 0' }}>
              <a href={`/feeds/${f.ID}/episodes`}>{f.Title ?? f.URL}</a>
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
