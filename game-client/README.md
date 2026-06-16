# Game Client

HTML5 Canvas game client for the multiplayer FPS. Connects to the game server via WebSocket, renders the game state at native frame rate, and sends player input back to the server.

## How It Works

1. Connects to the game server WebSocket at `ws://<host>:8080/ws`
2. Receives `GameStateMsg` each tick (player positions, kills feed, walls)
3. Renders the arena, players, and fog-of-war on a 1200x800 canvas
4. Captures keyboard (WASD) and mouse input, sends `InputState` to the server

## Controls

| Key | Action |
|-----|--------|
| W/A/S/D | Move up/left/down/right |
| Mouse | Aim direction |
| Left Click | Shoot |
| F5 | Toggle aimbot |
| F6 | Toggle speed hack |
| F7 | Toggle wallhack |
| F8 | Toggle triggerbot |

Cheat toggles (F5-F8) are for testing and dataset generation. They set flags on the player's input state, which the server uses to apply cheat behaviors and label telemetry accordingly.

## Features

- **Fog of war**: Players only see within a vision cone based on aim direction
- **Real-time rendering**: Canvas draws at requestAnimationFrame rate
- **Kill feed**: Recent kills displayed on screen
- **Auto-reconnect**: Reconnects to server on disconnect

## Running

The client is served as static files via Nginx:

```bash
# Via Docker Compose (recommended)
docker compose up game-client

# Standalone Nginx
docker run -p 80:80 -v $(pwd):/usr/share/nginx/html:ro nginx:alpine
```

Then open http://localhost:80 in a browser.

## Files

```
game-client/
  index.html    # Page structure, canvas element, HUD overlay
  game.js       # WebSocket client, rendering, input handling
  style.css     # Styling for canvas, HUD, connection overlay
  Dockerfile    # Nginx Alpine container
```
