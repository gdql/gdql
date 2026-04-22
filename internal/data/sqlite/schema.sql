-- GDQL schema (matches DATA_DESIGN.md)

CREATE TABLE IF NOT EXISTS venues (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    city TEXT,
    state TEXT,
    country TEXT,
    capacity INTEGER,
    latitude REAL,
    longitude REAL
);

CREATE TABLE IF NOT EXISTS shows (
    id INTEGER PRIMARY KEY,
    date TEXT NOT NULL,
    venue_id INTEGER REFERENCES venues(id),
    tour TEXT,
    notes TEXT,
    soundboard INTEGER,
    archive_id TEXT,
    rating REAL
);

CREATE TABLE IF NOT EXISTS songs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    short_name TEXT,
    writers TEXT,
    first_played TEXT,
    last_played TEXT,
    times_played INTEGER,
    is_cover INTEGER,
    original_artist TEXT,
    tempo_bpm INTEGER,
    typical_length_seconds INTEGER
);

CREATE TABLE IF NOT EXISTS performances (
    id INTEGER PRIMARY KEY,
    show_id INTEGER NOT NULL REFERENCES shows(id),
    song_id INTEGER NOT NULL REFERENCES songs(id),
    set_number INTEGER,
    position INTEGER NOT NULL,
    segue_type TEXT,
    length_seconds INTEGER,
    is_opener INTEGER,
    is_closer INTEGER,
    guest TEXT,
    notes TEXT,
    UNIQUE(show_id, set_number, position)
);

CREATE TABLE IF NOT EXISTS lyrics (
    song_id INTEGER PRIMARY KEY REFERENCES songs(id),
    lyrics TEXT,
    lyrics_fts TEXT
);

CREATE INDEX IF NOT EXISTS idx_shows_date ON shows(date);
CREATE TABLE IF NOT EXISTS song_aliases (
    alias TEXT PRIMARY KEY,
    song_id INTEGER NOT NULL REFERENCES songs(id)
);

-- Directed relations between two canonical songs. Distinct from song_aliases,
-- which normalizes raw setlist text into one canonical name. A relation
-- expresses that two already-canonical songs are connected:
--   merge_into  — same underlying song, prefer the target (data-cleanup cases)
--   variant_of  — distinct arrangement of the same tune (e.g. Minglewood Blues
--                 and New Minglewood Blues); keep both rows, cross-reference
--   pairs_with  — songs that commonly segue as a suite (Scarlet > Fire, etc.)
CREATE TABLE IF NOT EXISTS song_relations (
    from_song_id INTEGER NOT NULL REFERENCES songs(id),
    to_song_id INTEGER NOT NULL REFERENCES songs(id),
    kind TEXT NOT NULL CHECK (kind IN ('merge_into', 'variant_of', 'pairs_with')),
    PRIMARY KEY (from_song_id, to_song_id, kind),
    CHECK (from_song_id != to_song_id)
);
CREATE INDEX IF NOT EXISTS idx_song_relations_from ON song_relations(from_song_id);
CREATE INDEX IF NOT EXISTS idx_song_relations_to ON song_relations(to_song_id);

CREATE INDEX IF NOT EXISTS idx_songs_name ON songs(name);
CREATE INDEX IF NOT EXISTS idx_perf_song ON performances(song_id);
CREATE INDEX IF NOT EXISTS idx_perf_show ON performances(show_id);
CREATE INDEX IF NOT EXISTS idx_perf_position ON performances(show_id, set_number, position);
CREATE INDEX IF NOT EXISTS idx_shows_venue ON shows(venue_id);
