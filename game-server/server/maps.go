package server

import (
	"math"

	"cheat-detection/game-server/telemetry"
)

const (
	ArenaW = 1200
	ArenaH = 800
	WallT  = 20
)

type Vec2 struct {
	X, Y float64
}

type SpawnPoint struct {
	X, Y float64
}

// KillboxWalls returns the wall layout for "The Killbox" map.
func KillboxWalls() []telemetry.Wall {
	walls := []telemetry.Wall{
		// Outer border
		{X: 0, Y: 0, W: ArenaW, H: WallT},                  // top
		{X: 0, Y: ArenaH - WallT, W: ArenaW, H: WallT},     // bottom
		{X: 0, Y: 0, W: WallT, H: ArenaH},                   // left
		{X: ArenaW - WallT, Y: 0, W: WallT, H: ArenaH},      // right

		// NW corner room (top-left)
		{X: WallT, Y: 150, W: 200, H: WallT},                // bottom wall
		{X: 200, Y: WallT, W: WallT, H: 80},                 // right wall top segment
		// opening from y=100 to y=150
		// right wall is split for door

		// NE corner room (top-right)
		{X: ArenaW - 220, Y: 150, W: 200, H: WallT},         // bottom wall
		{X: ArenaW - 220, Y: WallT, W: WallT, H: 80},        // left wall top segment
		// opening from y=100 to y=150

		// SW corner room (bottom-left)
		{X: WallT, Y: ArenaH - 170, W: 200, H: WallT},       // top wall
		{X: 200, Y: ArenaH - 170, W: WallT, H: 80},          // right wall top segment (below top wall)
		// opening from y=(ArenaH-90) to y=(ArenaH-WallT)

		// SE corner room (bottom-right)
		{X: ArenaW - 220, Y: ArenaH - 170, W: 200, H: WallT}, // top wall
		{X: ArenaW - 220, Y: ArenaH - 170, W: WallT, H: 80},  // left wall top segment
		// opening from y=(ArenaH-90) to y=(ArenaH-WallT)

		// Central plaza (inner rectangle)
		{X: 400, Y: 280, W: 400, H: WallT},  // top
		{X: 400, Y: 500, W: 400, H: WallT},  // bottom
		{X: 400, Y: 280, W: WallT, H: 240},  // left
		{X: 780, Y: 280, W: WallT, H: 240},  // right

		// Top corridor dividers
		{X: 350, Y: 150, W: WallT, H: 130},  // left divider
		{X: 830, Y: 150, W: WallT, H: 130},  // right divider

		// Bottom corridor dividers
		{X: 350, Y: 520, W: WallT, H: 130},  // left divider
		{X: 830, Y: 520, W: WallT, H: 130},  // right divider
	}
	return walls
}

// KillboxSpawns returns spawn points, one in each corner room.
func KillboxSpawns() []SpawnPoint {
	return []SpawnPoint{
		{X: 100, Y: 80},                     // NW
		{X: ArenaW - 100, Y: 80},            // NE
		{X: 100, Y: ArenaH - 80},            // SW
		{X: ArenaW - 100, Y: ArenaH - 80},   // SE
	}
}

// RectContains checks if a circle (px, py, radius) overlaps with a wall rectangle.
func RectContains(wall telemetry.Wall, px, py, radius float64) bool {
	// Find the closest point on the rectangle to the circle center
	closestX := math.Max(wall.X, math.Min(px, wall.X+wall.W))
	closestY := math.Max(wall.Y, math.Min(py, wall.Y+wall.H))
	dx := px - closestX
	dy := py - closestY
	return (dx*dx + dy*dy) < (radius * radius)
}

// ResolveWallCollision pushes a circle out of all walls.
func ResolveWallCollision(walls []telemetry.Wall, px, py, radius float64) (float64, float64) {
	for _, wall := range walls {
		if !RectContains(wall, px, py, radius) {
			continue
		}
		// Compute penetration from each side
		left := (wall.X) - (px + radius)
		right := (wall.X + wall.W) - (px - radius)
		top := (wall.Y) - (py + radius)
		bottom := (wall.Y + wall.H) - (py - radius)

		// Find minimum absolute penetration
		minPen := right
		if math.Abs(left) < math.Abs(minPen) {
			minPen = left
		}
		if math.Abs(top) < math.Abs(minPen) {
			minPen = top
		}
		if math.Abs(bottom) < math.Abs(minPen) {
			minPen = bottom
		}

		if minPen == left || minPen == right {
			px += minPen
		} else {
			py += minPen
		}
	}
	return px, py
}

// LineIntersectsRect checks if a line segment intersects an AABB using slab method.
func LineIntersectsRect(x1, y1, x2, y2 float64, wall telemetry.Wall) bool {
	dx := x2 - x1
	dy := y2 - y1

	tMin := 0.0
	tMax := 1.0

	// Check X slab
	if math.Abs(dx) < 1e-9 {
		if x1 < wall.X || x1 > wall.X+wall.W {
			return false
		}
	} else {
		invD := 1.0 / dx
		t1 := (wall.X - x1) * invD
		t2 := (wall.X + wall.W - x1) * invD
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		tMin = math.Max(tMin, t1)
		tMax = math.Min(tMax, t2)
		if tMin > tMax {
			return false
		}
	}

	// Check Y slab
	if math.Abs(dy) < 1e-9 {
		if y1 < wall.Y || y1 > wall.Y+wall.H {
			return false
		}
	} else {
		invD := 1.0 / dy
		t1 := (wall.Y - y1) * invD
		t2 := (wall.Y + wall.H - y1) * invD
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		tMin = math.Max(tMin, t1)
		tMax = math.Min(tMax, t2)
		if tMin > tMax {
			return false
		}
	}

	return true
}

// HasLineOfSight returns true if no wall blocks the line between two points.
func HasLineOfSight(walls []telemetry.Wall, x1, y1, x2, y2 float64) bool {
	for _, wall := range walls {
		if LineIntersectsRect(x1, y1, x2, y2, wall) {
			return false
		}
	}
	return true
}

// AngleBetween returns the angle from (x1,y1) to (x2,y2) using atan2.
func AngleBetween(x1, y1, x2, y2 float64) float64 {
	return math.Atan2(y2-y1, x2-x1)
}

// AngleDiff returns the shortest angular difference (absolute value).
func AngleDiff(a, b float64) float64 {
	d := a - b
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	return math.Abs(d)
}

// Distance returns the euclidean distance between two points.
func Distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}
