# Podcatcher — Product Requirements Document

_2026-03-17_

---

## Overview

Podcatcher is a self-hosted podcast player that automatically detects and organises episodes into named series within a feed. It is built for listeners who follow podcasts that publish episodes from multiple distinct sub-series in a single RSS feed — non-chronologically and with inconsistent naming — a pattern that no mainstream podcast app handles well.

---

## Problem Statement

Podcast feeds increasingly publish episodes from multiple distinct, named sub-series within a single RSS feed — non-chronologically and with inconsistent naming conventions. Existing podcast apps treat every episode as a flat list, forcing listeners to manually track which series they're following, where they left off in each, and what they've already heard. This is a significant friction point for any listener who follows a podcast structured this way.

---

## Users

### Primary — The Self-Hosted Listener

A technically capable individual running their own software on personal infrastructure. They follow one or more podcasts where episodes from multiple named series are interleaved in a single feed. They are frustrated by mainstream podcast apps that offer no way to separate or track series independently. They are comfortable setting up a server but expect the day-to-day listening experience to feel polished and low-friction.

### Secondary — Household / Small Group _(Milestone 2)_

Multiple people sharing a single Podcatcher instance — for example, a household or small team with shared listening interests. Each person needs their own playback history and progress. This requires user accounts and authentication, which is out of scope for the POC but must not be architecturally precluded by early decisions.

---

## Goals & Success Criteria

| Goal | Success looks like |
|---|---|
| Series detection works without manual intervention | A listener adds a feed with mixed-series episodes and sees them correctly grouped within minutes, without any manual tagging |
| Handles real-world naming inconsistency | Detection correctly handles capitalisation differences, episodes with and without numbers, FINALE-only episodes, and prefix-number formats |
| Position is preserved across sessions | Reopening an episode resumes from exactly where the listener stopped |
| Portable | A listener can export their feed list as OPML and import it into any standard podcast app |
| Easy to self-host | A team member can stand up the server from a single binary and a Postgres database |

---

## Functional Requirements

### Feed Management

- Add and remove RSS feeds by URL
- Manually trigger a refresh on any feed
- Configure a per-feed poll interval (default: 1 hour)
- Import feeds from an OPML file
- Export the current feed list as an OPML file

### Series Detection

- Each feed has an ordered list of configurable regex rules stored alongside it
- Rules use named capture groups to extract the series name and episode number from episode titles
- Detection runs automatically at ingest time whenever new episodes are fetched
- Series names are matched case-insensitively to prevent duplicates from inconsistent capitalisation (e.g. "Miami Mince—Yule Regret It" and "Miami Mince—Yule Regret it" are the same series)
- An episode can belong to multiple series simultaneously — adding a series membership is additive and does not replace existing assignments
- Users can manually assign an episode to one or more series; manual assignments are never overwritten by automatic detection on subsequent fetches
- Users can remove a series assignment from an episode
- Users can create series manually
- Users can rename series

### Browsing & Discovery

Views:
- **All Episodes** — flat list across all feeds
- **By Show** — episodes grouped by feed
- **By Series** — episodes grouped by series within a feed
- **Unplayed** — episodes not yet completed
- **Played** — completed episodes

Sorting (available in all views):
- Publish date (default: newest first)
- Duration
- Title (alphabetical)

Filtering (composable):
- By feed
- By series
- By played / unplayed state
- By date range

### Playback

- Custom audio player with play/pause, seek bar, and playback speed control
- Playback position is saved continuously (on pause, on seek, and on a periodic heartbeat while playing)
- Position is restored automatically when an episode is reopened
- Episodes are marked as played when finished

### User Accounts _(Milestone 2)_

- Username/password registration and login
- Social sign-in via OAuth providers (Google, GitHub, Apple, and others)
- Per-user playback state, history, and preferences
- Admin role for managing feeds, rules, and users

---

## Non-Functional Requirements

### Self-Hosted

Podcatcher runs entirely on infrastructure the user controls. There is no cloud dependency, no telemetry, and no third-party account required to use it. The entire application ships as a single binary with an embedded frontend.

### Performance

- Feed polling and episode ingest run in the background and do not block the UI or API
- The player streams audio directly from source URLs — no audio is proxied through the server
- The application must remain responsive with hundreds of episodes across dozens of feeds

### Portability

OPML import/export ensures users are never locked in. Feed lists can be moved to or from any standard podcast app at any time. All data is stored in a Postgres database owned and controlled by the user.

### Security

- POC: no authentication required — single-user, trusted-network deployment assumed
- Milestone 2: standard session-based or token-based authentication; per-user data isolation enforced at the API layer; social sign-in via established OAuth providers

### Deployment

- Requires only a Postgres database and the compiled binary
- All configuration via environment variables
- No additional runtime dependencies

---

## Milestones

### Milestone 1 — POC _(current)_

**In scope:**
- Single-user, no authentication
- Feed management: add, remove, refresh, OPML import/export
- Background RSS poller with configurable interval
- Regex-based series detection with named capture groups, per-feed rule ordering, case-insensitive deduplication
- Additive series membership — episodes can belong to multiple series; manual assignments survive re-fetches
- Browsing with sorting and filtering as described above
- Custom audio player with position persistence and restore
- Single binary deployment (Go + embedded React SPA) backed by Postgres

### Milestone 2 — Multi-User & Auth

**In scope:**
- User accounts with username/password
- Social sign-in (Google, GitHub, Apple, and others)
- Per-user playback state, history, and preferences
- Admin role

### Milestone 3 — Native iOS App

**In scope:**
- Native iOS podcast player consuming the existing REST API
- The API is designed from Milestone 1 to support this without breaking changes

### Future _(not scheduled)_

The following are explicitly deferred and not ruled out for future milestones:

- Push notifications
- Chapter support
- Sleep timer
- Queue management
- Full-text episode search

---

## Out of Scope (all milestones)

- Cloud hosting or managed service
- Audio proxying through the server
- Enterprise / large-organisation multi-tenancy
