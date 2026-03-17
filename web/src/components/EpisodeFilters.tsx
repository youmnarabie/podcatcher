// web/src/components/EpisodeFilters.tsx
import type { EpisodeListParams } from '../types';

interface Props {
  params: EpisodeListParams;
  onChange: (p: EpisodeListParams) => void;
}

export default function EpisodeFilters({ params, onChange }: Props) {
  return (
    <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }}>
      <select value={params.sort ?? 'published_at'}
        onChange={e => onChange({ ...params, sort: e.target.value as EpisodeListParams['sort'] })}>
        <option value="published_at">Date</option>
        <option value="duration">Duration</option>
        <option value="title">Title</option>
      </select>
      <select value={params.order ?? 'desc'}
        onChange={e => onChange({ ...params, order: e.target.value as 'asc' | 'desc' })}>
        <option value="desc">Newest first</option>
        <option value="asc">Oldest first</option>
      </select>
      <select value={params.played === undefined ? '' : String(params.played)}
        onChange={e => {
          const v = e.target.value;
          onChange({ ...params, played: v === '' ? undefined : v === 'true' });
        }}>
        <option value="">All</option>
        <option value="false">Unplayed</option>
        <option value="true">Played</option>
      </select>
      <input type="date" placeholder="From"
        value={params.date_from?.slice(0, 10) ?? ''}
        onChange={e => onChange({ ...params, date_from: e.target.value ? e.target.value + 'T00:00:00Z' : undefined })} />
      <input type="date" placeholder="To"
        value={params.date_to?.slice(0, 10) ?? ''}
        onChange={e => onChange({ ...params, date_to: e.target.value ? e.target.value + 'T23:59:59Z' : undefined })} />
    </div>
  );
}
