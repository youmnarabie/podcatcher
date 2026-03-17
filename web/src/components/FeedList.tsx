// web/src/components/FeedList.tsx
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { api } from '../api';

export default function FeedList() {
  const qc = useQueryClient();
  const { data: feeds = [], isLoading } = useQuery({ queryKey: ['feeds'], queryFn: api.listFeeds });
  const [url, setUrl] = useState('');

  const addFeed = useMutation({
    mutationFn: (u: string) => api.createFeed(u),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['feeds'] }); setUrl(''); },
  });
  const deleteFeed = useMutation({
    mutationFn: api.deleteFeed,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['feeds'] }),
  });

  if (isLoading) return <p>Loading…</p>;
  return (
    <div>
      <h2>Shows</h2>
      <form onSubmit={e => { e.preventDefault(); addFeed.mutate(url); }}>
        <input value={url} onChange={e => setUrl(e.target.value)}
          placeholder="RSS feed URL" style={{ width: 400 }} />
        <button type="submit">Add</button>
      </form>
      <ul>
        {feeds.map(f => (
          <li key={f.ID}>
            <Link to={`/feeds/${f.ID}/episodes`}>{f.Title ?? f.URL}</Link>
            {' '}
            <button onClick={() => api.refreshFeed(f.ID)}>Refresh</button>
            {' '}
            <button onClick={() => deleteFeed.mutate(f.ID)}>Delete</button>
          </li>
        ))}
      </ul>
    </div>
  );
}
