#!/usr/bin/env python3
"""Fetch historic daily weather for every Grateful Dead show via Open-Meteo.

Input : gdql/data/venues_geo.json (slug -> {lat, lon})
        gdql embedded DB for show dates per venue
Output: gdql/data/weather.json, shape:
  {
    "1977-05-08": {
      "high_c": 15.3, "low_c": 7.9, "precip_mm": 0.0,
      "wind_kph": 14.4, "code": 1
    },
    ...
  }

Open-Meteo historical archive endpoint (no API key needed):
  https://archive-api.open-meteo.com/v1/archive
Batches dates per venue by passing start_date + end_date spanning the
venue's earliest and latest show. Pulls all days in between and keeps
only the dates that actually had a show. ~502 venues = ~502 calls.

Resume-safe: if weather.json exists, skip dates already covered.
"""
from __future__ import annotations

import json
import sqlite3
import sys
import time
import urllib.parse
import urllib.request
from collections import defaultdict
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
DB = ROOT / "run" / "embeddb" / "default.db"
GEO = ROOT / "data" / "venues_geo.json"
OUT = ROOT / "data" / "weather.json"
UA = "gdql-weather/0.1 (sam@burba.email)"
REQUEST_DELAY = 2.0  # Open-Meteo historical-archive enforces a sub-minute
                     # burst cap; 2s keeps us well under the sustained rate.
BACKOFF_ON_429 = 60  # wait out a minute when the throttle fires


def slugify(s: str) -> str:
    import re
    return re.sub(r"(^-|-$)", "", re.sub(r"[^a-z0-9]+", "-", (s or "").lower()))


def load_existing() -> dict:
    if not OUT.exists():
        return {}
    with open(OUT) as f:
        return json.load(f)


def save(data: dict):
    OUT.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT, "w") as f:
        json.dump(data, f, separators=(",", ":"))


def fetch_range(lat: float, lon: float, start: str, end: str, *, attempts: int = 4):
    params = {
        "latitude": lat,
        "longitude": lon,
        "start_date": start,
        "end_date": end,
        "daily": ",".join([
            "temperature_2m_max",
            "temperature_2m_min",
            "precipitation_sum",
            "wind_speed_10m_max",
            "weather_code",
        ]),
        "timezone": "auto",
    }
    url = "https://archive-api.open-meteo.com/v1/archive?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    for attempt in range(1, attempts + 1):
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.load(resp)
        except urllib.error.HTTPError as e:
            if e.code == 429 and attempt < attempts:
                wait = BACKOFF_ON_429 * attempt
                print(f"    429 — backing off {wait}s", file=sys.stderr)
                time.sleep(wait)
                continue
            raise


def main():
    if not DB.exists():
        sys.exit(f"gdql DB not found at {DB}")
    if not GEO.exists():
        sys.exit(f"venues_geo.json missing at {GEO} — run geocode_venues.py first")

    with open(GEO) as f:
        geo = json.load(f)

    conn = sqlite3.connect(DB)
    # Group show dates by venue slug.
    dates_by_venue: dict[str, list[str]] = defaultdict(list)
    for name, date in conn.execute(
        "SELECT v.name, s.date FROM shows s JOIN venues v ON v.id = s.venue_id "
        "WHERE s.date IS NOT NULL AND v.name IS NOT NULL"
    ):
        slug = slugify(name)
        if slug:
            dates_by_venue[slug].append(date)

    weather = load_existing()
    total_venues = len(dates_by_venue)
    fetched_venues = 0
    skipped_venues = 0
    days_added = 0

    for slug, dates in sorted(dates_by_venue.items()):
        rec = geo.get(slug, {})
        lat = rec.get("lat")
        lon = rec.get("lon")
        if lat is None or lon is None:
            skipped_venues += 1
            continue

        dates_needed = [d for d in dates if d not in weather]
        if not dates_needed:
            skipped_venues += 1
            continue

        start = min(dates_needed)
        end = max(dates_needed)

        try:
            time.sleep(REQUEST_DELAY)
            payload = fetch_range(lat, lon, start, end)
        except Exception as e:
            print(f"  [error] {slug} ({start}→{end}): {e}", file=sys.stderr)
            continue

        daily = payload.get("daily") or {}
        times = daily.get("time") or []
        highs = daily.get("temperature_2m_max") or []
        lows = daily.get("temperature_2m_min") or []
        precs = daily.get("precipitation_sum") or []
        winds = daily.get("wind_speed_10m_max") or []
        codes = daily.get("weather_code") or []

        wanted = set(dates_needed)
        for i, date in enumerate(times):
            if date not in wanted:
                continue
            weather[date] = {
                "high_c": highs[i] if i < len(highs) else None,
                "low_c": lows[i] if i < len(lows) else None,
                "precip_mm": precs[i] if i < len(precs) else None,
                "wind_kph": winds[i] if i < len(winds) else None,
                "code": codes[i] if i < len(codes) else None,
            }
            days_added += 1

        fetched_venues += 1
        if fetched_venues % 20 == 0:
            save(weather)
            print(
                f"  {fetched_venues}/{total_venues} venues "
                f"({days_added} days, {skipped_venues} cached/skipped)",
                file=sys.stderr,
            )

    save(weather)
    print(
        f"Done: {fetched_venues} venues fetched, {skipped_venues} cached/skipped, "
        f"{days_added} new days, {len(weather)} total days in weather.json",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
