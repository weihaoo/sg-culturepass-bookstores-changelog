import requests
from dotenv import load_dotenv
import json
import logging
import os
import time
from datetime import datetime
from pathlib import Path

logging.basicConfig(level=logging.INFO, format="%(message)s")
log = logging.getLogger(__name__)

# Env
load_dotenv()

API_URL = os.getenv("DEFAULT_API_URL")
SEARCH_URL = os.getenv("ONE_MAP_SEARCH_URL")
if not API_URL or API_URL == "__SET_ME__":
    raise RuntimeError("Missing required environment variable: DEFAULT_API_URL")
if not SEARCH_URL or SEARCH_URL == "__SET_ME__":
    raise RuntimeError("Missing required environment variable: ONE_MAP_SEARCH_URL")

# Config
DATA_FILE = "data/bookstores.json"
README_FILE = "README.md"
REQUEST_TIMEOUT = 30
GEOCODE_TIMEOUT = 10
RATE_LIMIT_DELAY = 0.1

CHANGELOG_START = "<!-- CHANGELOG_START -->"
CHANGELOG_END = "<!-- CHANGELOG_END -->"

# Helpers
def clean(value: object) -> str:
    if value is None:
        return ""
    return " ".join(str(value).split())

def esc(value: object) -> str:
    return clean(value).replace("|", "\\|")

# API
def fetch_bookstores() -> dict:
    resp = requests.get(
        API_URL,
        timeout=REQUEST_TIMEOUT,
        headers={
            "User-Agent": "Mozilla/5.0",
            "Accept": "application/json",
            "Origin": "https://www.sgculturepass.gov.sg",
            "Referer": "https://www.sgculturepass.gov.sg/",
        },
    )
    resp.raise_for_status()
    return resp.json()

def geocode(postal_code: str) -> tuple[float, float] | None:
    resp = requests.get(
        SEARCH_URL,
        params={"searchVal": postal_code, "returnGeom": "Y", "getAddrDetails": "Y", "pageNum": "1"},
        timeout=GEOCODE_TIMEOUT,
        headers={"User-Agent": "SG-CulturePass-Tracker/1.0", "Accept": "application/json"},
    )
    resp.raise_for_status()
    data = resp.json()
    results = data.get("results") or []
    if not results:
        log.warning("  No geocode results for postal code: %s", postal_code)
        return None
    return float(results[0]["LATITUDE"]), float(results[0]["LONGITUDE"])

# Data
def flatten(api_response: dict) -> list[dict]:
    data = api_response.get("data", {})
    locations, seen = [], set()

    def add(id, name, address, postal):
        if not id or id in seen or not address or not postal:
            return
        seen.add(id) # only accepts unique ids
        locations.append({"id": id, "name": name, "address": address, "postalCode": postal})

    # loop through the bookstore list
    for b in data.get("bookstores", []):
        # calls add function to try adding to locations list and seen set 
        add(clean(b.get("id")), clean(b.get("name")), clean(b.get("address")), clean(b.get("postalCode")))

    for brand, outlets in data.get("outlets", {}).items():
        for o in outlets:
            outlet = clean(o.get("outletName", ""))
            if brand.lower() in outlet.lower():
                name = outlet
            elif outlet:
                name = f"{brand} - {outlet}"
            else:
                name = brand
            add(clean(o.get("id")), name, clean(o.get("address")), clean(o.get("postalCode")))

    return locations

def load(path: str) -> list[dict]:
    with open(path, encoding="utf-8") as f:
        return json.load(f)

def save(path: str, locations: list[dict]) -> None:
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(locations, f, indent=2, ensure_ascii=False)
        f.write("\n")

def diff(old: list[dict], new: list[dict]) -> dict:
    old_map = {clean(l["id"]): l for l in old}
    added, changed = [], []

    for loc in new:
        id = clean(loc["id"])
        prev = old_map.get(id)

        if prev is None:
            added.append(loc)
            continue

        changes = [
            {"field": f, "oldValue": clean(prev.get(f)), "newValue": clean(loc.get(f))}
            for f in ("name", "address", "postalCode")
            if clean(prev.get(f)) != clean(loc.get(f))
        ]
        if changes:
            updated = dict(loc)
            if prev.get("latitude") is not None:
                updated["latitude"] = prev["latitude"]
                updated["longitude"] = prev["longitude"]
            changed.append({"location": updated, "changes": changes})

    return {"added": added, "changed": changed}

def geocode_locations(locations: list[dict], label: str = "") -> None:
    for loc in locations:
        postal = clean(loc.get("postalCode", ""))
        name = clean(loc.get("name", ""))
        if not postal:
            continue
        try:
            coords = geocode(postal)
            if coords is None:
                continue
            lat, lng = coords
            loc["latitude"], loc["longitude"] = lat, lng
            log.info("  %sGeocoded %s: %.6f, %.6f", label, name, lat, lng)
            time.sleep(RATE_LIMIT_DELAY)
        except requests.RequestException as e:
            log.warning("  Failed to geocode %s (%s): %s", name, postal, e)
        except (TypeError, ValueError, KeyError) as e:
            log.warning("  Invalid geocode response for %s (%s): %s", name, postal, e)

def merge(old: list[dict], new: list[dict], diff_result: dict) -> list[dict]:
    old_map = {clean(l["id"]): l for l in old}
    added_map = {clean(l["id"]): l for l in diff_result["added"]}
    changed_map = {clean(c["location"]["id"]): c["location"] for c in diff_result["changed"]}

    merged = []
    for loc in new:
        id = clean(loc["id"])
        if id in added_map:
            merged.append(added_map[id])
        elif id in changed_map:
            merged.append(changed_map[id])
        else:
            entry = dict(loc)
            if prev := old_map.get(id):
                if prev.get("latitude") is not None:
                    entry["latitude"] = prev["latitude"]
                    entry["longitude"] = prev["longitude"]
            merged.append(entry)

    return merged

# Changelog

def added_table(locations: list[dict]) -> str:
    rows = ["| Name | Address | Postal Code |", "|------|---------|-------------|"]
    for l in locations:
        rows.append(f"| {esc(l.get('name'))} | {esc(l.get('address'))} | {esc(l.get('postalCode'))} |")
    return "\n".join(rows) + "\n"

def changed_cell(field: str, value: object, changes: list[dict]) -> str:
    for c in changes:
        if c["field"] == field:
            return f"~~{esc(c['oldValue'])}~~ <br> {esc(c['newValue'])}"
    return esc(value)

def changed_table(changed: list[dict]) -> str:
    rows = ["| Name | Address | Postal Code |", "|------|---------|-------------|"]
    for c in changed:
        loc, changes = c["location"], c["changes"]
        rows.append(
            f"| {changed_cell('name', loc.get('name'), changes)} | "
            f"{changed_cell('address', loc.get('address'), changes)} | "
            f"{changed_cell('postalCode', loc.get('postalCode'), changes)} |"
        )
    return "\n".join(rows) + "\n"

def format_entry(diff_result: dict, old_count: int, new_count: int) -> str:
    date = datetime.now().strftime("%Y-%m-%d")
    added, changed = diff_result["added"], diff_result["changed"]
    out = [f"<details open>\n<summary><strong>{date}</strong> (Total locations: {old_count} -> {new_count})</summary>\n\n"]

    if added:
        out += [f"<ul><li><details><summary>Added ({len(added)})</summary>\n\n", added_table(added), "\n</details></li></ul>\n\n"]
    if changed:
        out += [f"<ul><li><details><summary>Changed ({len(changed)})</summary>\n\n", changed_table(changed), "\n</details></li></ul>\n\n"]

    out.append("</details>\n\n")
    return "".join(out)

def update_readme(entry: str) -> None:
    content = Path(README_FILE).read_text(encoding="utf-8")
    start = content.index(CHANGELOG_START) + len(CHANGELOG_START)
    end = content.index(CHANGELOG_END)
    existing = content[start:end].strip()
    normalized_entry = entry.strip()

    if existing and normalized_entry in existing:
        return

    if existing:
        changelog = "\n" + entry + existing + "\n"
    else:
        changelog = "\n" + entry

    Path(README_FILE).write_text(content[:start] + changelog + content[end:], encoding="utf-8")

# Main
def main() -> None:
    log.info("Fetching bookstores...")
    try:
        api_response = fetch_bookstores()
    except requests.RequestException as e:
        log.error("Failed to fetch bookstores: %s", e)
        return
    except ValueError as e:
        log.error("Invalid bookstore API response: %s", e)
        return

    new_locations = flatten(api_response)
    log.info("Found %d locations", len(new_locations))

    try:
        old_locations = load(DATA_FILE)
        log.info("Loaded %d previous locations", len(old_locations))
    except FileNotFoundError:
        log.info("No previous state (first run)")
        old_locations = []

    diff_result = diff(old_locations, new_locations)

    if not diff_result["added"] and not diff_result["changed"]:
        log.info("No changes. Done.")
        return

    log.info("+%d added, ~%d changed", len(diff_result["added"]), len(diff_result["changed"]))

    if diff_result["added"]:
        log.info("Geocoding new locations...")
        geocode_locations(diff_result["added"])

    if diff_result["changed"]:
        log.info("Re-geocoding changed locations...")
        needs_geocode = [
            c["location"] for c in diff_result["changed"]
            if any(ch["field"] == "postalCode" for ch in c["changes"])
        ]
        geocode_locations(needs_geocode, label="Re-")

    merged = merge(old_locations, new_locations, diff_result)

    update_readme(format_entry(diff_result, len(old_locations), len(new_locations)))
    save(DATA_FILE, merged)
    log.info("Done.")


if __name__ == "__main__":
    main()
