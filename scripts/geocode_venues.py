#!/usr/bin/env python3
"""Geocode every distinct venue in gdql to lat/lon via Nominatim.

Output: gdql/data/venues_geo.json, shape:
  {"<slug>": {"name": "...", "city": "...", "state": "...",
              "lat": 42.4489, "lon": -76.4799}, ...}

Resume-safe: skips slugs already present in the output file. Nominatim
asks for 1 req/sec and a contact in the User-Agent string.
"""
from __future__ import annotations

import json
import os
import re
import sqlite3
import sys
import time
import urllib.parse
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
DB = ROOT / "run" / "embeddb" / "default.db"
OUT = ROOT / "data" / "venues_geo.json"
UA = "gdql-geocoder/0.1 (sam@burba.email)"

def slugify(s: str) -> str:
    return re.sub(r"(^-|-$)", "", re.sub(r"[^a-z0-9]+", "-", (s or "").lower()))

def load_existing() -> dict:
    if not OUT.exists():
        return {}
    with open(OUT) as f:
        return json.load(f)

def save(data: dict):
    OUT.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT, "w") as f:
        json.dump(data, f, indent=2, sort_keys=True)

def geocode(query: str):
    url = (
        "https://nominatim.openstreetmap.org/search?"
        + urllib.parse.urlencode({"q": query, "format": "json", "limit": 1})
    )
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = json.load(resp)
    except Exception as e:
        return None, str(e)
    if not data:
        return None, "no_result"
    top = data[0]
    return (float(top["lat"]), float(top["lon"])), None

def main():
    if not DB.exists():
        sys.exit(f"gdql DB not found at {DB}")

    conn = sqlite3.connect(DB)
    # Aggregate show counts per (venue_name, city, state) from the canonical
    # DB; minor spelling variants that share a slug get merged under one key.
    rows = list(conn.execute("""
        SELECT v.name, v.city, v.state, count(s.id) as show_count
        FROM venues v LEFT JOIN shows s ON s.venue_id = v.id
        WHERE v.name IS NOT NULL AND v.name != ''
        GROUP BY v.id
        ORDER BY show_count DESC
    """).fetchall())

    # Also ingest venue names from shows_with_lengths.json — that file was
    # produced by a separate pipeline and carries ~200 venue spellings the
    # DB's venues table doesn't have. Dedup by slug.
    sw = ROOT / "shows_with_lengths.json"
    if sw.exists():
        import json as _json
        seen_slugs = {slugify(n) for n, *_ in rows}
        extra: dict[str, tuple[str, str, str, int]] = {}
        for show in _json.load(open(sw)):
            v = show.get("venue") or {}
            name = (v.get("name") or "").strip()
            if not name:
                continue
            slug = slugify(name)
            if not slug or slug in seen_slugs:
                continue
            city = (v.get("city") or "").strip()
            state = (v.get("state") or "").strip()
            prev = extra.get(slug)
            count = (prev[3] + 1) if prev else 1
            extra[slug] = (name, city, state, count)
        rows.extend(extra.values())
        print(
            f"shows_with_lengths: +{len(extra)} extra venues merged into queue",
            file=sys.stderr,
        )

    existing = load_existing()
    added = 0
    skipped = 0
    no_match = 0

    for name, city, state, count in rows:
        slug = slugify(name)
        if not slug:
            continue
        if slug in existing:
            skipped += 1
            continue

        # Build a geocoding query biased toward the right locale. Strip
        # anything after a comma in city (gdql has "Ithaca, NY, USA" style).
        city_clean = (city or "").split(",")[0].strip()
        parts = [name]
        if city_clean:
            parts.append(city_clean)
        if state:
            parts.append(state)
        query = ", ".join(parts)

        time.sleep(1.05)  # be polite to Nominatim (1 req/sec limit)
        coords, err = geocode(query)
        if coords is None:
            # Retry with just the city
            if city_clean:
                time.sleep(1.05)
                coords, err = geocode(", ".join([city_clean, state or ""]).strip(", "))
        if coords is None:
            no_match += 1
            print(f"  [no match] {query} — {err}", file=sys.stderr)
            existing[slug] = {
                "name": name,
                "city": city_clean,
                "state": state or "",
                "show_count": count,
            }
            if added % 20 == 0:
                save(existing)
            continue

        existing[slug] = {
            "name": name,
            "city": city_clean,
            "state": state or "",
            "show_count": count,
            "lat": coords[0],
            "lon": coords[1],
        }
        added += 1
        if added % 10 == 0:
            save(existing)
            print(f"  {added} geocoded, {no_match} missed, {skipped} cached", file=sys.stderr)

    save(existing)
    print(f"Done: {added} geocoded, {no_match} missed, {skipped} cached, "
          f"{len(existing)} total", file=sys.stderr)

if __name__ == "__main__":
    main()
