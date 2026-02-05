-- GDQL test schema (matches DATA_DESIGN.md)

CREATE TABLE venues (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    city TEXT,
    state TEXT,
    country TEXT,
    capacity INTEGER,
    latitude REAL,
    longitude REAL
);

CREATE TABLE shows (
    id INTEGER PRIMARY KEY,
    date TEXT NOT NULL,
    venue_id INTEGER REFERENCES venues(id),
    tour TEXT,
    notes TEXT,
    soundboard INTEGER,
    archive_id TEXT,
    rating REAL
);

CREATE TABLE songs (
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

CREATE TABLE performances (
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

CREATE TABLE lyrics (
    song_id INTEGER PRIMARY KEY REFERENCES songs(id),
    lyrics TEXT,
    lyrics_fts TEXT
);

CREATE TABLE song_aliases (
    alias TEXT PRIMARY KEY,
    song_id INTEGER NOT NULL REFERENCES songs(id)
);

CREATE INDEX idx_shows_date ON shows(date);
CREATE INDEX idx_songs_name ON songs(name);
CREATE INDEX idx_perf_song ON performances(song_id);
CREATE INDEX idx_perf_show ON performances(show_id);
CREATE INDEX idx_perf_position ON performances(show_id, set_number, position);
CREATE INDEX idx_shows_venue ON shows(venue_id);
