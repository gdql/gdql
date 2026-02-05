#!/usr/bin/env python3
"""
Populate a GDQL canonical JSON file from good sources (Relisten API, optional Archive.org).

Output can be imported with:  gdql import json <output.json>

Usage:
  pip install requests
  python scripts/populate_canonical_json.py -o shows.json
  python scripts/populate_canonical_json.py -o shows.json --years 1972 1977 1978
  python scripts/populate_canonical_json.py -o shows.json --first-year 1965 --last-year 1995
"""

import argparse
import json
import re
import sys
import time
from urllib.parse import quote

try:
    import requests
except ImportError:
    print("pip install requests", file=sys.stderr)
    sys.exit(1)

# Relisten: https://api.relisten.net or https://relistenapi.alecgorge.com (see RelistenNet/RelistenApi on GitHub)
RELISTEN_BASE = "https://api.relisten.net/api/v2"
ARTIST_SLUG = "grateful-dead"


def parse_segue_titles(track_title):
    """Split 'Scarlet Begonias > Fire on the Mountain' into [('Scarlet Begonias', False), ('Fire on the Mountain', True)]."""
    parts = re.split(r"\s*>\s*", track_title.strip())
    out = []
    for i, name in enumerate(parts):
        name = name.strip()
        if name:
            out.append((name, i > 0))  # segue_before true for all but first
    return out if out else [(track_title.strip(), False)]


def normalize_date(s):
    """Return YYYY-MM-DD from API date (e.g. 1977-05-08T00:00:00Z or 1977-05-08)."""
    s = (s or "").strip()
    if len(s) >= 10 and s[4] == "-" and s[7] == "-":
        return s[:10]
    return s


def relisten_show_to_canonical(show):
    """Convert one Relisten show payload to canonical Show dict."""
    date_str = show.get("date") or show.get("display_date") or ""
    if not date_str:
        return None
    date = normalize_date(date_str)

    venue = show.get("venue") or {}
    if isinstance(venue, dict):
        name = (venue.get("name") or "").strip() or "Unknown"
        location = (venue.get("location") or "").strip()
        city = (venue.get("city") or "").strip()
        state = (venue.get("state") or "").strip()
        country = (venue.get("country") or "").strip()
        if not city and location:
            city = location  # Relisten uses "location" e.g. "Van Nuys, CA"
        v = {"name": name, "city": city, "state": state, "country": country}
    else:
        v = {"name": "Unknown", "city": "", "state": "", "country": ""}

    sets_out = []
    sources = show.get("sources") or show.get("source") or []
    if not isinstance(sources, list):
        sources = [sources] if sources else []
    # Some APIs put sets directly on show
    if not sources and show.get("sets"):
        sources = [{"sets": show["sets"]}]
    # Relisten returns many sources (recordings) per show; use first source for one setlist
    if sources:
        sources = sources[:1]

    for src in sources:
        raw_sets = src.get("sets") or src.get("tracks") or []
        if not isinstance(raw_sets, list):
            raw_sets = [raw_sets] if raw_sets else []
        for s in raw_sets:
            tracks = s.get("tracks") if isinstance(s.get("tracks"), list) else (s.get("songs") or [])
            if not tracks and isinstance(s, dict) and "title" in s:
                tracks = [s]
            if not tracks and isinstance(s, list):
                tracks = s
            songs = []
            for t in tracks or []:
                title = t.get("title") or t.get("song") or (t if isinstance(t, str) else "")
                if isinstance(t, dict) and not title:
                    title = t.get("name") or ""
                title = (title or "").strip()
                if not title:
                    continue
                for name, segue_before in parse_segue_titles(title):
                    songs.append({"name": name, "segue_before": segue_before})
            if songs:
                sets_out.append({"songs": songs})

    if not sets_out:
        return None

    return {
        "date": date,
        "venue": v,
        "tour": (show.get("tour") or {}).get("name", "") if isinstance(show.get("tour"), dict) else (show.get("tour") or ""),
        "notes": show.get("notes") or "",
        "sets": sets_out,
    }


def fetch_relisten_years(years, session, delay=0.3):
    """Fetch all shows for the given years from Relisten."""
    base_url = f"{RELISTEN_BASE}/artists/{ARTIST_SLUG}"
    all_shows = []
    for year in years:
        time.sleep(delay)
        url = f"{base_url}/years/{year}"
        try:
            r = session.get(url, timeout=30)
            r.raise_for_status()
            data = r.json()
        except Exception as e:
            print(f"Warning: Relisten years/{year}: {e}", file=sys.stderr)
            continue
        # Response may be { "shows": [ ... ] } or direct list
        shows = data.get("shows") or data.get("data") or (data if isinstance(data, list) else [])
        for s in shows:
            all_shows.append(s)
        print(f"Relisten {year}: {len(shows)} shows", file=sys.stderr)
    return all_shows


def fetch_relisten_show_details(show_list, session, delay=0.2):
    """Fetch full show details (with sources/sets) for each show date."""
    base_url = f"{RELISTEN_BASE}/artists/{ARTIST_SLUG}/shows"
    out = []
    for i, show in enumerate(show_list):
        date_str = show.get("date") or show.get("display_date") or show.get("id")
        if not date_str:
            canonical = relisten_show_to_canonical(show)
            if canonical:
                out.append(canonical)
            continue
        date_for_url = normalize_date(date_str)
        time.sleep(delay)
        url = f"{base_url}/{date_for_url}"
        try:
            r = session.get(url, timeout=30)
            r.raise_for_status()
            detail = r.json()
            # Response may be { "show": { ... } } or the show object directly
            if isinstance(detail, dict) and "show" in detail:
                detail = detail["show"]
            canonical = relisten_show_to_canonical(detail)
            if canonical:
                out.append(canonical)
        except Exception:
            # If year list already had full data, use it
            canonical = relisten_show_to_canonical(show)
            if canonical:
                out.append(canonical)
        if (i + 1) % 50 == 0:
            print(f"  fetched {i + 1}/{len(show_list)} shows", file=sys.stderr)
    return out


def fetch_archive_etree(year, session, delay=0.5, max_items=200):
    """Fetch Grateful Dead items from Archive.org etree for a year. Best-effort setlist from metadata."""
    # advancedsearch: collection:etree and creator:"Grateful Dead" and date
    q = f"collection:etree AND creator:Grateful Dead AND date:{year}"
    url = "https://archive.org/advancedsearch.php"
    params = {"q": q, "fl": ["identifier", "date", "venue", "title"], "output": "json", "rows": max_items, "page": 1}
    time.sleep(delay)
    try:
        r = session.get(url, params=params, timeout=30)
        r.raise_for_status()
        data = r.json()
    except Exception as e:
        print(f"Warning: Archive.org search {year}: {e}", file=sys.stderr)
        return []
    docs = (data.get("response") or {}).get("docs") or []
    out = []
    for doc in docs:
        date_str = (doc.get("date") or "").strip()
        if not date_str or len(date_str) < 8:
            continue
        # Normalize YYYYMMDD or YYYY-MM-DD
        if len(date_str) == 8 and date_str.isdigit():
            date_str = f"{date_str[:4]}-{date_str[4:6]}-{date_str[6:8]}"
        venue_name = (doc.get("venue") or doc.get("title") or "Unknown").strip()
        if isinstance(venue_name, list):
            venue_name = (venue_name[0] or "Unknown").strip()
        # Archive often doesn't give us track list in search; we'd need item metadata. Skip or add one placeholder set.
        # So we only add a stub show if we want to merge with Relisten later; for now skip archive if no tracks.
        # Option: fetch item metadata for identifier to get tracklist - would need extra request per item.
    return out


def main():
    ap = argparse.ArgumentParser(description="Populate GDQL canonical JSON from Relisten (and optional Archive.org)")
    ap.add_argument("-o", "--output", default="shows.json", help="Output JSON file")
    ap.add_argument("--years", type=int, nargs="+", help="Specific years (e.g. 1972 1977 1978)")
    ap.add_argument("--first-year", type=int, default=1965, help="First year (with --last-year)")
    ap.add_argument("--last-year", type=int, default=1995, help="Last year")
    ap.add_argument("--relisten-only", action="store_true", help="Only use Relisten API")
    ap.add_argument("--delay", type=float, default=0.25, help="Delay between API requests (seconds)")
    args = ap.parse_args()

    if args.years:
        years = sorted(set(args.years))
    else:
        years = list(range(args.first_year, args.last_year + 1))

    session = requests.Session()
    session.headers.setdefault("User-Agent", "GDQL-populate/1.0 (https://github.com/gdql/gdql)")

    print("Fetching from Relisten API...", file=sys.stderr)
    show_list = fetch_relisten_years(years, session, delay=args.delay)
    if not show_list:
        print("No shows from Relisten. Check API or try --years 1977", file=sys.stderr)
        sys.exit(1)

    print("Fetching show details (setlists)...", file=sys.stderr)
    canonical_list = fetch_relisten_show_details(show_list, session, delay=args.delay)
    if not canonical_list:
        # Fallback: use year list as-is (sometimes it has sets inline)
        for s in show_list:
            c = relisten_show_to_canonical(s)
            if c:
                canonical_list.append(c)

    # Dedupe by date+venue
    seen = set()
    unique = []
    for c in canonical_list:
        key = (c["date"], c["venue"].get("name", ""), c["venue"].get("city", ""))
        if key in seen:
            continue
        seen.add(key)
        unique.append(c)

    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(unique, f, indent=2, ensure_ascii=False)

    print(f"Wrote {len(unique)} shows to {args.output}", file=sys.stderr)
    print(f"Import with: gdql import json {args.output}", file=sys.stderr)


if __name__ == "__main__":
    main()
