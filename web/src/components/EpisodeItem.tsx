// web/src/components/EpisodeItem.tsx
import type { Episode } from '../types';

interface Props { episode: Episode; onPlay?: (ep: Episode) => void; }

export default function EpisodeItem({ episode, onPlay }: Props) {
  const date = episode.PublishedAt ? new Date(episode.PublishedAt).toLocaleDateString() : '';
  const dur = episode.DurationSeconds
    ? `${Math.floor(episode.DurationSeconds / 60)}m` : '';
  return (
    <div style={{ borderBottom: '1px solid #eee', padding: '8px 0' }}>
      <strong>{episode.Title}</strong>
      {date && <small style={{ marginLeft: 8 }}>{date}</small>}
      {dur && <small style={{ marginLeft: 8 }}>{dur}</small>}
      <br />
      <button onClick={() => onPlay?.(episode)}>▶ Play</button>
    </div>
  );
}
