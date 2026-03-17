CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE feeds (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  url                   TEXT NOT NULL UNIQUE,
  title                 TEXT,
  description           TEXT,
  image_url             TEXT,
  poll_interval_seconds INT NOT NULL DEFAULT 3600,
  last_fetched_at       TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE feed_rules (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feed_id    UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  pattern    TEXT NOT NULL,
  priority   INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE episodes (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feed_id            UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid               TEXT NOT NULL,
  title              TEXT NOT NULL,
  description        TEXT,
  audio_url          TEXT NOT NULL,
  duration_seconds   INT,
  published_at       TIMESTAMPTZ,
  raw_season         TEXT,
  raw_episode_number TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(feed_id, guid)
);

CREATE TABLE series (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feed_id    UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(feed_id, name)
);

CREATE TABLE series_episodes (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  series_id          UUID NOT NULL REFERENCES series(id) ON DELETE CASCADE,
  episode_id         UUID NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
  episode_number     INT,
  is_manual_override BOOL NOT NULL DEFAULT FALSE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(series_id, episode_id)
);

CREATE TABLE playback_state (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  episode_id       UUID NOT NULL REFERENCES episodes(id) ON DELETE CASCADE UNIQUE,
  position_seconds INT NOT NULL DEFAULT 0,
  completed        BOOL NOT NULL DEFAULT FALSE,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
