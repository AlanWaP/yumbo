# Yumbo Multiplayer Framework

Yumbo is a lightweight multiplayer web framework. The frontend is a static page
that can be hosted on GitHub Pages, while the realtime backend runs separately
on your PC or another host.

This first version is intentionally game-agnostic:

- The frontend has connection, lobby, queue, room status, and an empty game
  frame.
- The backend handles WebSocket connections, player IDs, game-type queues,
  room creation, room leaving, disconnect cleanup, and generic room messages.
- Game-specific UI and rules can be added later as separate modules.

## Local Development

Start the backend:

```sh
go run .
```

The backend listens at:

```text
ws://localhost:3000
```

Serve the frontend from another terminal:

```sh
python3 -m http.server 8080
```

Open:

```text
http://localhost:8080
```

Open two browser tabs, connect both to `ws://localhost:3000`, enter the same
game type, and both players will be matched into a room.

## GitHub Pages

GitHub Pages can host only the static frontend files:

- `index.html`
- `style.css`
- `script.js`

When using the hosted frontend at:

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

- `join_queue` with `gameType`
- `leave_queue`
- `leave_room`
- `room_message` with arbitrary `payload`

Server messages:

- `connected`
- `queued`
- `already_queued`
- `room_created`
- `queue_left`
- `not_queued`
- `room_left`
- `peer_left`
- `room_message`
- `error`

The backend does not interpret game-specific payloads. It only relays
`room_message.payload` to the other players in the same room.

## Tests

Run backend tests:

```sh
go test ./...
```
