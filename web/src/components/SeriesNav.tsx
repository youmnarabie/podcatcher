// web/src/components/SeriesNav.tsx
import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { api } from '../api';

export default function SeriesNav() {
  const { feedId } = useParams<{ feedId: string }>();
  const { data: series = [] } = useQuery({
    queryKey: ['series', feedId],
    queryFn: () => api.listSeries(feedId!),
    enabled: !!feedId,
  });
  if (!feedId || series.length === 0) return null;
  return (
    <div style={{ marginBottom: 12 }}>
      <strong>Series: </strong>
      {series.map(s => (
        <Link key={s.ID} to={`/feeds/${feedId}/series/${s.ID}`} style={{ marginRight: 8 }}>
          {s.Name}
        </Link>
      ))}
    </div>
  );
}
