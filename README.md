# Yumbo Multiplayer Framework

Yumbo is a lightweight multiplayer web framework. The frontend is a static page
that can be hosted on GitHub Pages, while the realtime backend runs separately
on your PC or another host.

This first version is intentionally game-agnostic:

- The frontend has connection, dropdown game creation, an existing-games lobby,
  queue, room status, and an empty game frame under `frontend/`.
- The backend handles WebSocket connections, player IDs, game-type queues,
  configurable room sizes, room creation, room leaving, disconnect cleanup, and
  generic room messages under `backend/`.
- Game-specific UI and rules can be added later as separate modules.

## Project Layout

```text
backend/   Go WebSocket backend and tests
frontend/  Static HTML, CSS, and browser JavaScript
scripts/   Local development helpers
```

## Local Development

Requirements:

- Go
- Python 3

Start everything with one command:

```sh
./scripts/dev_local.sh
```

The script starts the backend and frontend, opens the local page on macOS, and
stops both servers when you press Ctrl-C.

By default, the backend listens at:

```text
ws://localhost:3000
```

and the frontend runs at:

```text
http://localhost:8080/?server=ws://localhost:3000
```

The `server` query parameter lets the frontend connect to the local WebSocket
backend automatically.

You can override the ports, backend URL, or browser launch:

```sh
PORT=3001 FRONTEND_PORT=8081 OPEN_BROWSER=0 ./scripts/dev_local.sh
```

Available environment variables:

- `PORT`: backend WebSocket port, default `3000`
- `FRONTEND_PORT`: static frontend port, default `8080`
- `SERVER_URL`: backend URL passed to the frontend, default `ws://localhost:$PORT`
- `OPEN_BROWSER`: set to `0` to skip opening the browser, default `1`

Open multiple browser tabs, connect them to `ws://localhost:3000`, create a
game from the dropdowns, or join a waiting game from the existing-games list.
Players are matched into a room when enough players join the same game type and
player count.

## GitHub Pages

GitHub Pages can host only the static frontend files:

- `frontend/index.html`
- `frontend/style.css`
- `frontend/script.js`

Publish the static files from `frontend/`. When using the hosted frontend at:

```text
https://alanwap.github.io/yumbo/
```

the backend still needs to run somewhere else. For testing from your PC, expose
the local backend through a secure tunnel such as Cloudflare Tunnel or ngrok and
use the resulting `wss://` URL.

You can also pass the backend URL with a query parameter:

```text
https://alanwap.github.io/yumbo/?server=wss://example.trycloudflare.com
```

## Backend Protocol

Client messages:

- `join_queue` with `gameType` and optional `playerCount`
- `leave_queue`
- `leave_room`
- `request_lobby`
- `game_move` with `payload.moveType`; generic games use `attack`, `defend`,
  or `gain_power`, while game-specific modules can define their own moves
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
- `game_move_accepted`
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
