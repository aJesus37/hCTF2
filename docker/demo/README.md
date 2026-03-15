# hCTF2 Demo

Self-contained demo environment for hCTF2 that resets every 30 minutes.

## Quick Start

```bash
docker compose -f docker/demo/docker-compose.yml up --build
```

Then open http://localhost:8090.

## What You Get

- **Admin account**: `admin@demo.hctf2` / `Admin123!`
- **5 demo users**: alice, bob, carol, dave, eve (`@demo.hctf2`, password: `demo123`)
- **3 teams**: Shadow Hackers, Binary Wolves, Crypto Ninjas
- **8 challenges** across web, crypto, forensics, and misc categories
- **Pre-populated scoreboard** with staggered submissions over the past 2 hours
- **MOTD** on the login page showing credentials and next reset time

## Auto-Reset

The demo state (database) is wiped and re-seeded every 30 minutes.
The MOTD is updated with the next reset time after each cycle.

## Architecture

A single Alpine-based container runs:

1. The hCTF2 binary (Go server)
2. A seed script that populates data via the REST API
3. A background loop that resets every 30 minutes

The seed script creates all data through the API to ensure correct password
hashing and UUID generation, then uses sqlite3 to backdate submission
timestamps so the score evolution chart shows realistic progression.
