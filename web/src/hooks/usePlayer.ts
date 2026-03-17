// web/src/hooks/usePlayer.ts
import { Howl } from 'howler';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api';
import type { Episode } from '../types';

const HEARTBEAT_MS = 10_000;

export function usePlayer() {
  const [episode, setEpisode] = useState<Episode | null>(null);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [speed, setSpeed] = useState(1);
  const howl = useRef<Howl | null>(null);
  const hb = useRef<ReturnType<typeof setInterval> | null>(null);

  const save = useCallback((ep: Episode, pos: number, completed: boolean) => {
    api.upsertPlayback(ep.ID, Math.floor(pos), completed).catch(() => {});
  }, []);

  const stopHB = () => { if (hb.current) clearInterval(hb.current); };

  const play = useCallback((ep: Episode, startPos = 0) => {
    howl.current?.unload();
    stopHB();
    const h = new Howl({
      src: [ep.AudioURL], html5: true, rate: speed,
      onload: () => { setDuration(h.duration()); h.seek(startPos); h.play(); },
      onplay: () => setPlaying(true),
      onpause: () => { setPlaying(false); save(ep, h.seek() as number, false); },
      onend: () => { setPlaying(false); save(ep, h.duration(), true); stopHB(); },
    });
    howl.current = h;
    setEpisode(ep);
    setPosition(startPos);
    hb.current = setInterval(() => {
      if (h.playing()) { const p = h.seek() as number; setPosition(p); save(ep, p, false); }
    }, HEARTBEAT_MS);
  }, [speed, save]);

  const togglePlay = useCallback(() => {
    if (!howl.current) return;
    howl.current.playing() ? howl.current.pause() : howl.current.play();
  }, []);

  const seek = useCallback((seconds: number) => {
    if (!howl.current || !episode) return;
    howl.current.seek(seconds);
    setPosition(seconds);
    save(episode, seconds, false);
  }, [episode, save]);

  const setPlaybackSpeed = useCallback((s: number) => {
    setSpeed(s); howl.current?.rate(s);
  }, []);

  useEffect(() => {
    const id = setInterval(() => {
      if (howl.current?.playing()) setPosition(howl.current.seek() as number);
    }, 500);
    return () => clearInterval(id);
  }, []);

  useEffect(() => () => { howl.current?.unload(); stopHB(); }, []);

  return { episode, playing, position, duration, speed, play, togglePlay, seek, setPlaybackSpeed };
}
