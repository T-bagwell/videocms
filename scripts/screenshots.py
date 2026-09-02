#!/usr/bin/env python3
"""Capture UI screenshots for docs/screenshots.

Requirements: a running backend (API on :8080), a running frontend (Vite on
:5173 by default), and demo data seeded into the "演示媒体库" library (see
scripts/make-demo-media.sh and scripts/make-demo-media-extra.sh).

Usage:
    python3 scripts/screenshots.py

Environment overrides:
    VIDEOCMS_API   API base URL        (default http://localhost:8080/api)
    VIDEOCMS_UI    UI base URL         (default http://localhost:5173)
"""

import json
import os
import sys
import time
import urllib.request

from playwright.sync_api import sync_playwright


API = os.environ.get("VIDEOCMS_API", "http://localhost:8080/api").rstrip("/")
UI = os.environ.get("VIDEOCMS_UI", "http://localhost:5173").rstrip("/")
OUT = os.path.normpath(os.path.join(os.path.dirname(__file__), "..", "docs", "screenshots"))
DEMO_LIBRARY = os.environ.get("DEMO_LIBRARY", "演示媒体库")
ADMIN_USER = os.environ.get("ADMIN_USER", "admin")
ADMIN_PASS = os.environ.get("ADMIN_PASS", "admin123")


def api(method, path, token=None, body=None):
    req = urllib.request.Request(API + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode() if body is not None else None
    with urllib.request.urlopen(req, data=data) as resp:
        return json.load(resp)


def main():
    token = api("POST", "/auth/login", body={"username": ADMIN_USER, "password": ADMIN_PASS})["token"]
    libs = api("GET", "/libraries", token)["items"]
    demo = next((l for l in libs if l["name"] == DEMO_LIBRARY), None)
    if not demo:
        sys.exit(f"demo library {DEMO_LIBRARY!r} not found")
    videos = api("GET", f"/videos?library_id={demo['id']}&page_size=100", token)["items"]
    feat = next((v for v in videos if "Interstellar" in v.get("filename", "")), videos[0])
    second = next((v for v in videos if "Inception" in v.get("filename", "")), feat)
    series = api("GET", f"/series?library_id={demo['id']}", token)["items"]
    show = next((s for s in series if "Demo Show" in s.get("name", "")), series[0] if series else None)
    albums = api("GET", "/albums", token)["items"]
    books = api("GET", "/books", token)["items"]
    photo_albums = api("GET", "/photo-albums", token)["items"]
    shares = api("GET", f"/videos/{feat['id']}/shares", token).get("items", [])
    if not shares:
        created = api("POST", f"/videos/{feat['id']}/share", token, {"expires_hours": 168})
        shares = [created]
    share_token = shares[0]["token"]

    os.makedirs(OUT, exist_ok=True)
    shots = []

    def snap(page, name, wait_ms=1200):
        page.wait_for_timeout(wait_ms)
        page.screenshot(path=os.path.join(OUT, name), full_page=False)
        shots.append(name)
        print("captured", name)

    def wait_any(page, *selectors, timeout=30000):
        page.wait_for_function(
            "sel => sel.some(s => document.querySelector(s))",
            arg=list(selectors),
            timeout=timeout,
        )

    def capture(page, path, name, *selectors, wait_ms=1200):
        for attempt in (1, 2):
            try:
                page.goto(UI + path, wait_until="networkidle", timeout=60000)
                if selectors:
                    wait_any(page, *selectors)
                snap(page, name, wait_ms)
                return
            except Exception as exc:  # noqa: BLE001
                print("retry", name, f"(attempt {attempt}):", exc)
                page.wait_for_timeout(2000)
        # best effort: capture whatever rendered
        try:
            page.screenshot(path=os.path.join(OUT, name), full_page=False)
            shots.append(name)
            print("captured (best effort)", name)
        except Exception:  # noqa: BLE001
            pass

    def select_library(page):
        for sel in page.locator("select").all():
            labels = sel.locator("option").all_inner_texts()
            if DEMO_LIBRARY in labels:
                sel.select_option(label=DEMO_LIBRARY)
                return True
        return False

    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page(viewport={"width": 1440, "height": 900}, device_scale_factor=2)

        # login page
        page.goto(UI + "/login", wait_until="networkidle", timeout=60000)
        snap(page, "login.png", 600)
        page.evaluate("(t) => localStorage.setItem('videocms_token', t)", token)
        page.goto(UI + "/", wait_until="networkidle", timeout=60000)
        page.wait_for_selector(".navbar", timeout=30000)

        # home: filter to the demo library for a clean grid
        page.goto(UI + "/", wait_until="networkidle", timeout=60000)
        wait_any(page, ".card-grid", ".empty")
        select_library(page)
        page.wait_for_timeout(2500)
        snap(page, "home.png")

        # series
        page.goto(UI + "/series", wait_until="networkidle", timeout=60000)
        wait_any(page, ".video-grid", ".card-grid", ".empty")
        select_library(page)
        page.wait_for_timeout(2000)
        snap(page, "series.png")
        if show:
            capture(page, f"/series/{show['id']}", "series-detail.png", ".detail-info")

        # video detail + player
        capture(page, f"/video/{feat['id']}", "detail.png", ".detail-info")
        page.goto(UI + f"/player/{feat['id']}", wait_until="networkidle", timeout=60000)
        page.wait_for_selector(".player-wrap video", timeout=30000)
        page.wait_for_timeout(6000)
        snap(page, "player.png")

        # music
        capture(page, "/music", "music.png", ".album-card")
        if albums:
            capture(page, f"/music/{albums[0]['id']}", "music-album.png", ".playlist-row", ".empty")

        # photos
        capture(page, "/photos", "photos.png", ".album-card", ".empty")
        if photo_albums:
            capture(page, f"/photos/{photo_albums[0]['id']}", "photos-album.png", ".photo-cell", ".empty")

        # books
        capture(page, "/books", "books.png", ".book-cover", ".empty")
        if books:
            epub = next((b for b in books if b["format"] == "epub"), books[0])
            page.goto(UI + f"/books/{epub['id']}", wait_until="networkidle", timeout=60000)
            page.wait_for_selector(".reader-frame", timeout=30000)
            page.wait_for_timeout(3000)
            snap(page, "book-reader.png")

        # stats, subscriptions, requests, settings
        capture(page, "/stats", "stats.png", ".stats-grid", ".empty")
        capture(page, "/subscriptions", "subscriptions.png", ".video-grid", ".card-grid", ".empty")
        capture(page, "/requests", "requests.png", "form.card.admin-tools", ".empty")
        capture(page, "/settings", "settings.png", "form.card.admin-tools", ".empty")

        # public pages
        capture(page, "/public", "public.png", ".video-grid", ".card-grid", ".empty")
        page.goto(UI + f"/share/{share_token}", wait_until="networkidle", timeout=60000)
        page.wait_for_selector(".share-page, .share-head, .player", timeout=30000)
        page.wait_for_timeout(3000)
        snap(page, "share.png")

        # admin tabs
        capture(page, "/admin", "admin.png", ".tabs")
        admin_tabs = [
            ("Libraries", "admin-libraries.png"),
            ("Videos", "admin-videos.png"),
            ("Users", "admin-users.png"),
            ("Uploads", "admin-uploads.png"),
            ("Downloads", "admin-downloads.png"),
            ("Storage", "admin-storage.png"),
            ("Jobs", "admin-jobs.png"),
            ("Webhooks", "admin-webhooks.png"),
            ("IPTV", "admin-iptv.png"),
            ("Requests", "admin-requests.png"),
            ("Quality", "admin-quality.png"),
            ("Invites", "admin-invites.png"),
            ("Recordings", "admin-recordings.png"),
            ("Trakt", "admin-trakt.png"),
            ("Moderation", "admin-moderation.png"),
            ("Scrapers", "admin-scrapers.png"),
            ("Plugins", "admin-plugins.png"),
            ("Transcode", "admin-transcode.png"),
        ]
        for label, name in admin_tabs:
            try:
                page.click(f".tabs button:text-is('{label}')")
                page.wait_for_timeout(1500)
                snap(page, name, 500)
            except Exception as exc:  # noqa: BLE001
                print("skip", label, exc)

        browser.close()

    print(f"\n{len(shots)} screenshots written to {OUT}")


if __name__ == "__main__":
    main()
