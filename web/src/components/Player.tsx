// web/src/components/Player.tsx
import type { Episode } from '../types';

interface Props {
  episode: Episode | null; playing: boolean; position: number;
  duration: number; speed: number;
  onToggle: () => void; onSeek: (s: number) => void; onSpeedChange: (s: number) => void;
}

function fmt(s: number) {
  const m = Math.floor(s / 60), sec = Math.floor(s % 60);
  return `${m}:${sec.toString().padStart(2, '0')}`;
}

export default function Player({ episode, playing, position, duration, speed, onToggle, onSeek, onSpeedChange }: Props) {
  if (!episode) return null;
  return (
    <div style={{ position: 'fixed', bottom: 0, left: 0, right: 0, background: '#222', color: '#fff', padding: 12 }}>
      <div style={{ maxWidth: 900, margin: '0 auto', display: 'flex', alignItems: 'center', gap: 12 }}>
        <button onClick={onToggle} style={{ fontSize: 20 }}>{playing ? '⏸' : '▶'}</button>
        <span style={{ minWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {episode.Title}
        </span>
        <span>{fmt(position)}</span>
        <input type="range" min={0} max={duration || 1} step={1} value={position}
          onChange={e => onSeek(Number(e.target.value))} style={{ flex: 1 }} />
        <span>{fmt(duration)}</span>
        <select value={speed} onChange={e => onSpeedChange(Number(e.target.value))}>
          {[0.75, 1, 1.25, 1.5, 1.75, 2].map(s => <option key={s} value={s}>{s}×</option>)}
        </select>
      </div>
    </div>
  );
}
