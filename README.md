# Yumbo Multiplayer Framework

Yumbo is a lightweight multiplayer web framework. The frontend is a static page
that can be hosted on GitHub Pages, while the realtime backend runs separately
on your PC or another host.

The framework is game-agnostic, with pluggable game modules:

- The frontend has connection, dropdown game creation, an existing-games lobby,
  queue, room status, and a game frame under `frontend/`. The first module is
  Power, Defense and Wave (`power_defense_wave`).
- The backend handles WebSocket connections, player IDs, game-type queues,
  configurable room sizes, room creation, room leaving, disconnect cleanup, and
  generic room messages under `backend/`.
- Additional game-specific UI and rules can be added as separate modules.

## Project Layout

```text
backend/   Go WebSocket backend and tests
frontend/  Static HTML, CSS, and browser JavaScript
scripts/   dev_local.sh for local dev, start-backend.sh for GitHub Pages testing
```

## Local Development

Use `scripts/dev_local.sh` to run the full stack on your machine: Go backend,
static frontend, and an auto-opened browser tab.

### `dev_local.sh`

Requirements:

- Go
- Python 3

Run:

```sh
./scripts/dev_local.sh
```

What it does:

1. Checks that the backend and frontend ports are free (via `port_check.sh`).
2. Starts the Go backend with `go run ./backend`.
3. Serves `frontend/` with `python3 -m http.server`.
4. Opens the local page in your browser on macOS (when `open` is available).
5. Keeps both processes running until you press Ctrl-C, then stops them together.

By default:

```text
Backend:  ws://localhost:3000
Frontend: http://localhost:8080/?server=ws://localhost:3000
```

The `server` query parameter tells the frontend which WebSocket backend to use.

Override ports, backend URL, or browser launch:

```sh
PORT=3001 FRONTEND_PORT=8081 OPEN_BROWSER=0 ./scripts/dev_local.sh
```

Environment variables:

- `PORT`: backend WebSocket port, default `3000`
- `FRONTEND_PORT`: static frontend port, default `8080`
- `SERVER_URL`: backend URL passed to the frontend, default `ws://localhost:$PORT`
- `OPEN_BROWSER`: set to `0` to skip opening the browser, default `1`

Open multiple browser tabs against the same frontend URL, create a game from the
dropdowns, or join a waiting game from the existing-games list. Players are
matched into a room when enough players join the same game type and player count.

## GitHub Pages

GitHub Pages can host only the static frontend. This repo publishes from the
repository root:

- `index.html` (entry page with `<base href="frontend/" />`)
- `.nojekyll`
- everything under `frontend/` (`index.html`, `style.css`, `script.js`,
  `i18n.js`, `waiting-room.js`, `game-screen.js`)

The hosted frontend is at:

```text
https://AlanWaP.github.io/yumbo/
```

The backend still needs to run on your PC or another host. Browsers loading the
hosted page require a secure `wss://` URL, not `ws://localhost`.

### `start-backend.sh`

Use `scripts/start-backend.sh` when you want to play against the GitHub Pages
frontend while running the backend locally.

Requirements:

- Go
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/)

Run:

```sh
./scripts/start-backend.sh
```

What it does:

1. Checks that the backend port is free (via `port_check.sh`).
2. Starts the Go backend with `go run ./backend`.
3. Starts a Cloudflare Tunnel to expose the backend over HTTPS/WSS.
4. Waits for the tunnel URL, converts it to `wss://`, and prints a ready-to-open
   GitHub Pages link with the `server` query parameter filled in.
5. Keeps the backend and tunnel running until you press Ctrl-C, then stops both.

Example output URL:

```text
https://AlanWaP.github.io/yumbo/?server=wss://example.trycloudflare.com
```

Use a different backend port if needed:

```sh
PORT=3001 ./scripts/start-backend.sh
```

Override the hosted frontend URL if you are testing a fork or preview deploy:

```sh
PAGES_URL=https://yourname.github.io/yumbo/ ./scripts/start-backend.sh
```

To run only the backend without a tunnel (for example with `./scripts/dev_local.sh`):

```sh
PORT=3000 go run ./backend
```

You can also start your own tunnel (Cloudflare, ngrok, etc.) and pass the
resulting `wss://` URL manually:

```text
https://AlanWaP.github.io/yumbo/?server=wss://example.trycloudflare.com
```

## Backend Protocol

Client messages:

- `join_queue` with `gameType` and optional `playerCount`
- `leave_queue`
- `leave_room`
- `request_lobby`
- `game_move` with `payload.moveType`; generic games use `attack`, `defend`,
  or `gain_power`, while game-specific modules can define their own moves
- `cancel_move`
- `refresh_pending` (keeps queue/room state during a page refresh)
- `leave_session` (disconnect without immediately dropping queue/room state)
- `room_message` with arbitrary `payload`

Server messages:

- `connected`
- `lobby_update` with `games`
- `queued`
- `already_queued`
- `room_created`
- `queue_left`
- `not_queued`
- `room_left`
- `peer_left`
- `peer_disconnected`
- `peer_reconnected`
- `game_move_accepted`
- `game_move_cancelled`
- `game_state`
- `round_resolved`
- `game_finished`
- `room_message`
- `error`

If `playerCount` is omitted, the backend defaults to 2. `join_queue` also
accepts optional `gameMode` as `free_for_all` or `team`. Team games currently
use two teams and require an even player count. The backend matches players only
when `gameType`, `playerCount`, and game mode settings are the same.

Each `lobby_update.games` item includes:

- `id`
- `status` as `waiting` or `started`
- `gameType`
- `gameMode`
- `teamCount`
- `playerCount`
- `joinedPlayerCount`
- `players`

Rooms now include a basic authoritative round system that future games can
extend. Each alive player submits one move per round:

- `attack` targets an enemy player and spends power.
- `defend` blocks incoming attack damage for the round.
- `gain_power` increases power for future attacks.

The backend resolves a round after every alive player moves, then broadcasts the
new game state. `room_message.payload` remains available as an opaque extension
channel for game-specific UI messages.

The first game-specific module is `power_defense_wave`, shown as "Power,
Defense and Wave" in the frontend. Players start at 0 power and can choose:

- `power`: gain 1 power, but lose if attacked that round.
- `defense`: block attacks unless multiple players use `super_blast`; cannot
  be used three rounds in a row.
- `wave`: spend 1 power to attack one target.
- `super_blast`: spend 3 power to attack all enemy players.
- `air_cannon`: target one player; if that player uses `super_blast`, they lose,
  but their blast still resolves.

## Tests

Run backend tests:

```sh
go test ./...
```
