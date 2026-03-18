import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';

export default function SearchBar() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [value, setValue] = useState(() => searchParams.get('q') ?? '');

  // Keep input in sync when URL changes (e.g. browser back/forward)
  useEffect(() => {
    setValue(searchParams.get('q') ?? '');
  }, [searchParams]);

  // Debounced navigation
  useEffect(() => {
    const timer = setTimeout(() => {
      if (value) {
        navigate('/search?q=' + encodeURIComponent(value), { replace: true });
      } else {
        navigate('/', { replace: true });
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [value, navigate]);

  return (
    <input
      type="search"
      placeholder="Search episodes & shows…"
      aria-label="Search episodes and shows"
      value={value}
      onChange={e => setValue(e.target.value)}
      style={{ marginLeft: 'auto' }}
    />
  );
}
