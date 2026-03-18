// web/src/App.tsx
import { Route, Routes, NavLink } from 'react-router-dom';
import FeedList from './components/FeedList';
import EpisodeList from './components/EpisodeList';
import SeriesNav from './components/SeriesNav';
import Player from './components/Player';
import SearchBar from './components/SearchBar';
import SearchResults from './components/SearchResults';
import { usePlayer } from './hooks/usePlayer';
import { api } from './api';
import type { Episode } from './types';

export default function App() {
  const player = usePlayer();

  const handlePlay = async (ep: Episode) => {
    let startPos = 0;
    try {
      const pb = await api.getPlayback(ep.ID);
      if (!pb.Completed) startPos = pb.PositionSeconds;
    } catch {}
    player.play(ep, startPos);
  };

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: 16, paddingBottom: 80 }}>
      <nav style={{ marginBottom: 16, display: 'flex', gap: 16 }}>
        <NavLink to="/">All Episodes</NavLink>
        <NavLink to="/feeds">Shows</NavLink>
        <NavLink to="/unplayed">Unplayed</NavLink>
        <SearchBar />
      </nav>
      <Routes>
        <Route path="/" element={<EpisodeList onPlay={handlePlay} />} />
        <Route path="/feeds" element={<FeedList />} />
        <Route path="/feeds/:feedId/episodes" element={
          <><SeriesNav /><EpisodeList onPlay={handlePlay} /></>
        } />
        <Route path="/feeds/:feedId/series/:seriesId" element={
          <><SeriesNav /><EpisodeList onPlay={handlePlay} /></>
        } />
        <Route path="/unplayed" element={<EpisodeList played={false} onPlay={handlePlay} />} />
        <Route path="/search" element={<SearchResults onPlay={handlePlay} />} />
      </Routes>
      <Player
        episode={player.episode} playing={player.playing}
        position={player.position} duration={player.duration} speed={player.speed}
        onToggle={player.togglePlay} onSeek={player.seek}
        onSpeedChange={player.setPlaybackSpeed}
      />
    </div>
  );
}
