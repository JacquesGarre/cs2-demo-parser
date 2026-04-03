# CS2 Demo Parser

CS2 Demo Parser is a full-stack project to upload and analyze Counter-Strike 2 demos, then visualize match insights in a web dashboard. The stack includes:

- Backend API in Go
- Frontend in Angular
- PostgreSQL for persistence
- Docker Compose orchestration, driven by Make targets

## Local Setup

### Prerequisites

- Git
- Docker Desktop (or Docker Engine + Compose plugin)
- GNU Make (`make`)

### Start

```bash
git clone https://github.com/JacquesGarre/cs2-demo-parser.git
cd cs2-demo-parser
git checkout main
make local-start
```

This builds and starts:

- Postgres on `localhost:5432`
- Backend API on `localhost:8080`
- Frontend on `http://localhost:4200`

## Local Development Commands

Run from the project root:

```bash
make help          # List all targets
make local-start   # Build and start postgres, backend, frontend
make local-stop    # Stop and remove containers
make local-restart # Restart all services
make local-logs    # Follow service logs
make local-ps      # Show running service status
make local-rebuild # Force rebuild and restart
make local-clean   # Stop services and remove volumes
```

## TODOs

### UI / Radar

- [ ] Fix Inferno radar minimap offset
- [x] Fix Nuke two-levels radar
- [x] Fix first opening of drawer not selecting the proper player in dropdown
- [ ] Add ui tooltip over kills and death in radar to know which round and which side it happened
- [ ] Add minimap replay per round with utility + kills animation (2D top-down)
- [ ] Add more players stats in the drawer (multikills) et "rounds phares"
- [ ] Add a 404 page, and why reloading the page doesn't work?

### Match Insights

- [ ] Add FIFA-style player cards with ratings
- [ ] Add deeper round analysis with timeline:
   - Team average money per player
   - Buy type classification (with tooltip details)
   - Team setup classification (rush, execute, hold, fake, etc.)
   - Round event timeline (entries, plants, clutches, saves, defuses)
   - Round ending context (plant/defuse/time), key fight zones, setups, clutches, entries, trades
- [ ] Add highlight video per player
- [ ] Add highlight video per round
- [ ] Add round win-odds estimation based on buys
- [ ] Add user accounts, team creation, store demos per team to get an average win %
- [ ] For teams, name tactics, and see the % of wins over maps and rounds
- [ ] Wire faceit demos directly