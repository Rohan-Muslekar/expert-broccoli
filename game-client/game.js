const canvas = document.getElementById('game');
const ctx = canvas.getContext('2d');
const overlay = document.getElementById('overlay');

const W = 1200, H = 800;
const PLAYER_R = 15;
const BG_COLOR = '#1a1a2e';
const WALL_COLOR = '#555';

let ws = null;
let walls = [];
let state = { tick: 0, players: [], projectiles: [], kills: [] };
let myId = null;

const keys = { w: false, a: false, s: false, d: false };
const mouse = { x: W / 2, y: H / 2 };
let shooting = false;
const cheats = { aimbot: false, speedhack: false, wallhack: false, triggerbot: false };

function connect() {
    overlay.classList.remove('hidden');
    overlay.querySelector('p').textContent = 'Connecting...';

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = location.hostname || 'localhost';
    ws = new WebSocket(`${protocol}//${host}:8080/ws`);

    ws.onopen = () => {
        overlay.classList.add('hidden');
    };

    ws.onmessage = (e) => {
        const msg = JSON.parse(e.data);
        if (msg.walls && msg.walls.length > 0) {
            walls = msg.walls;
        }
        if (msg.players) {
            state = msg;
        }
        if (!myId && msg.players && msg.players.length > 0) {
            for (const p of msg.players) {
                if (p.id.startsWith('player-')) {
                    myId = p.id;
                    break;
                }
            }
        }
    };

    ws.onclose = () => {
        overlay.classList.remove('hidden');
        overlay.querySelector('p').textContent = 'Disconnected. Reconnecting...';
        setTimeout(connect, 2000);
    };

    ws.onerror = () => ws.close();
}

document.addEventListener('keydown', (e) => {
    const k = e.key.toLowerCase();
    if (k in keys) keys[k] = true;
    if (e.key === 'F5') { e.preventDefault(); cheats.aimbot = !cheats.aimbot; }
    if (e.key === 'F6') { e.preventDefault(); cheats.speedhack = !cheats.speedhack; }
    if (e.key === 'F7') { e.preventDefault(); cheats.wallhack = !cheats.wallhack; }
    if (e.key === 'F8') { e.preventDefault(); cheats.triggerbot = !cheats.triggerbot; }
});

document.addEventListener('keyup', (e) => {
    const k = e.key.toLowerCase();
    if (k in keys) keys[k] = false;
});

canvas.addEventListener('mousemove', (e) => {
    const rect = canvas.getBoundingClientRect();
    mouse.x = e.clientX - rect.left;
    mouse.y = e.clientY - rect.top;
});

canvas.addEventListener('mousedown', () => { shooting = true; });
canvas.addEventListener('mouseup', () => { shooting = false; });

function sendInput() {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ keys, mouse, shooting, cheats }));
    }
}

function drawFogOfWar(px, py, aim) {
    const FOV = Math.PI / 2;
    const RANGE = 300;
    const RAYS = 120;

    ctx.save();

    const points = [];
    for (let i = 0; i <= RAYS; i++) {
        const angle = aim - FOV / 2 + (FOV / RAYS) * i;

        let minDist = RANGE;
        for (const w of walls) {
            const d = rayRectDist(px, py, angle, w);
            if (d !== null && d < minDist) {
                minDist = d;
            }
        }
        points.push({
            x: px + Math.cos(angle) * minDist,
            y: py + Math.sin(angle) * minDist
        });
    }

    ctx.fillStyle = 'rgba(0, 0, 0, 0.9)';
    ctx.beginPath();
    ctx.rect(0, 0, W, H);

    ctx.moveTo(px, py);
    for (const p of points) {
        ctx.lineTo(p.x, p.y);
    }
    ctx.closePath();
    ctx.fill('evenodd');

    ctx.restore();
}

function rayRectDist(ox, oy, angle, w) {
    const dx = Math.cos(angle);
    const dy = Math.sin(angle);
    let tMin = 0, tMax = 10000;

    if (Math.abs(dx) > 1e-9) {
        let t1 = (w.x - ox) / dx;
        let t2 = (w.x + w.w - ox) / dx;
        if (t1 > t2) [t1, t2] = [t2, t1];
        tMin = Math.max(tMin, t1);
        tMax = Math.min(tMax, t2);
    } else if (ox < w.x || ox > w.x + w.w) {
        return null;
    }

    if (Math.abs(dy) > 1e-9) {
        let t1 = (w.y - oy) / dy;
        let t2 = (w.y + w.h - oy) / dy;
        if (t1 > t2) [t1, t2] = [t2, t1];
        tMin = Math.max(tMin, t1);
        tMax = Math.min(tMax, t2);
    } else if (oy < w.y || oy > w.y + w.h) {
        return null;
    }

    if (tMin > tMax || tMax < 0) return null;
    return tMin > 0 ? tMin : tMax > 0 ? 0 : null;
}

function render() {
    ctx.fillStyle = BG_COLOR;
    ctx.fillRect(0, 0, W, H);

    ctx.fillStyle = WALL_COLOR;
    for (const w of walls) {
        ctx.fillRect(w.x, w.y, w.w, w.h);
    }

    for (const p of state.players) {
        ctx.beginPath();
        ctx.arc(p.x, p.y, PLAYER_R, 0, Math.PI * 2);
        ctx.fillStyle = p.alive ? (p.id === myId ? '#4488ff' : '#44ff44') : '#442222';
        ctx.fill();
        if (!p.alive) {
            ctx.strokeStyle = '#ff4444';
            ctx.lineWidth = 2;
            ctx.stroke();
        }

        if (p.alive) {
            ctx.beginPath();
            ctx.moveTo(p.x, p.y);
            ctx.lineTo(p.x + Math.cos(p.aim) * 30, p.y + Math.sin(p.aim) * 30);
            ctx.strokeStyle = p.shooting ? '#ffff00' : '#ffffff44';
            ctx.lineWidth = 2;
            ctx.stroke();
        }

        if (p.alive) {
            const barW = 30, barH = 4;
            const barX = p.x - barW / 2, barY = p.y - PLAYER_R - 8;
            ctx.fillStyle = '#333';
            ctx.fillRect(barX, barY, barW, barH);
            const hpFrac = p.hp / 100;
            const r = Math.floor(255 * (1 - hpFrac));
            const g = Math.floor(255 * hpFrac);
            ctx.fillStyle = `rgb(${r},${g},0)`;
            ctx.fillRect(barX, barY, barW * hpFrac, barH);
        }

        ctx.fillStyle = '#aaa';
        ctx.font = '10px monospace';
        ctx.textAlign = 'center';
        ctx.fillText(p.id, p.x, p.y - PLAYER_R - 12);
    }

    const me = state.players.find(p => p.id === myId);
    if (me && me.alive) {
        drawFogOfWar(me.x, me.y, me.aim);
    }

    if (state.kills) {
        ctx.fillStyle = '#ff6644';
        ctx.font = '12px monospace';
        ctx.textAlign = 'right';
        for (let i = 0; i < state.kills.length; i++) {
            const k = state.kills[i];
            ctx.fillText(`${k.killer} > ${k.victim}`, W - 10, 20 + i * 16);
        }
    }

    document.getElementById('cheat-f5').classList.toggle('active', cheats.aimbot);
    document.getElementById('cheat-f6').classList.toggle('active', cheats.speedhack);
    document.getElementById('cheat-f7').classList.toggle('active', cheats.wallhack);
    document.getElementById('cheat-f8').classList.toggle('active', cheats.triggerbot);
}

function gameLoop() {
    sendInput();
    render();
    requestAnimationFrame(gameLoop);
}

connect();
requestAnimationFrame(gameLoop);
