#!/usr/bin/env python3
"""
Merge track durations from Relisten API into an existing GDQL database.

Matches by show date + track position within each set. Does NOT replace
song names or set structure — only adds length_seconds where missing.

Usage:
  python scripts/merge_durations.py -db shows.db
  python scripts/merge_durations.py -db shows.db --first-year 1965 --last-year 1995
"""

import argparse
import json
import re
import sqlite3
import sys
import time

try:
    import requests
except ImportError:
    print("pip install requests", file=sys.stderr)
    sys.exit(1)

RELISTEN_BASE = "https://api.relisten.net/api/v2"
ARTIST_SLUG = "grateful-dead"


def normalize_date(s):
    s = (s or "").strip()
    if len(s) >= 10 and s[4] == "-" and s[7] == "-":
        return s[:10]
    return s


def fetch_relisten_show(date_str, session):
    """Fetch a single show from Relisten by date. Returns list of (set_num, position, duration_seconds) tuples."""
    url = f"{RELISTEN_BASE}/artists/{ARTIST_SLUG}/shows/{date_str}"
    try:
        r = session.get(url, timeout=30)
        if r.status_code == 404:
            return []
        r.raise_for_status()
        data = r.json()
    except Exception as e:
        print(f"  warn: {date_str}: {e}", file=sys.stderr)
        return []

    sources = data.get("sources", [])
    if not sources:
        return []

    tracks = []
    for set_idx, s in enumerate(sources[0].get("sets", []), 1):
        set_num = min(set_idx, 3)  # cap at 3 (encore)
        for pos, t in enumerate(s.get("tracks", []), 1):
            duration = int(t.get("duration") or 0)
            title = (t.get("title") or "").strip()
            if duration > 0 and title:
                # Handle segue-combined tracks: "Scarlet Begonias-> Fire On The Mountain"
                parts = re.split(r"\s*-?>\s*", title)
                per_song = duration // len(parts) if len(parts) > 1 else duration
                for i, part in enumerate(parts):
                    tracks.append((set_num, pos + i if len(parts) > 1 else pos, per_song, part.strip()))
    return tracks


def main():
    parser = argparse.ArgumentParser(description="Merge Relisten durations into GDQL DB")
    parser.add_argument("-db", required=True, help="Path to GDQL database")
    parser.add_argument("--first-year", type=int, default=1965)
    parser.add_argument("--last-year", type=int, default=1995)
    args = parser.parse_args()

    conn = sqlite3.connect(args.db)
    cur = conn.cursor()

    # Get all show dates
    cur.execute("SELECT id, date FROM shows ORDER BY date")
    shows = cur.fetchall()
    print(f"{len(shows)} shows in DB", file=sys.stderr)

    # Filter to year range
    shows = [(sid, d) for sid, d in shows
             if args.first_year <= int(d[:4]) <= args.last_year]
    print(f"{len(shows)} in year range {args.first_year}-{args.last_year}", file=sys.stderr)

    session = requests.Session()
    session.headers["User-Agent"] = "gdql-duration-merge/1.0"

    updated = 0
    skipped = 0
    no_match = 0

    for i, (show_id, show_date) in enumerate(shows):
        time.sleep(0.2)  # rate limit

        relisten_tracks = fetch_relisten_show(show_date, session)
        if not relisten_tracks:
            skipped += 1
            continue

        # Get performances for this show
        cur.execute("""
            SELECT p.id, p.set_number, p.position, s.name, p.length_seconds
            FROM performances p
            JOIN songs s ON p.song_id = s.id
            WHERE p.show_id = ?
            ORDER BY p.set_number, p.position
        """, (show_id,))
        perfs = cur.fetchall()

        # Match by set_number + position
        relisten_by_pos = {}
        for set_num, pos, dur, title in relisten_tracks:
            relisten_by_pos[(set_num, pos)] = (dur, title)

        for perf_id, set_num, position, song_name, existing_len in perfs:
            if existing_len and existing_len > 0:
                continue  # already has duration

            key = (set_num, position)
            if key in relisten_by_pos:
                dur, _ = relisten_by_pos[key]
                cur.execute("UPDATE performances SET length_seconds = ? WHERE id = ?", (dur, perf_id))
                updated += 1
            else:
                no_match += 1

        if (i + 1) % 100 == 0:
            conn.commit()
            print(f"  {i + 1}/{len(shows)} shows processed, {updated} durations added", file=sys.stderr)

    conn.commit()
    conn.close()
    print(f"\nDone: {updated} durations added, {skipped} shows not on Relisten, {no_match} tracks unmatched", file=sys.stderr)


if __name__ == "__main__":
    main()
