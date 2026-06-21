package telemetry

type PlayerTelemetry struct {
	Timestamp           int64   `json:"ts"`
	PlayerID            string  `json:"pid"`
	Tick                int64   `json:"tick"`
	PosX                float64 `json:"x"`
	PosY                float64 `json:"y"`
	VelX                float64 `json:"vx"`
	VelY                float64 `json:"vy"`
	AimAngle            float64 `json:"aim"`
	AimDelta            float64 `json:"aim_delta"`
	IsShooting          bool    `json:"shooting"`
	HitTarget           bool    `json:"hit"`
	Health              int     `json:"hp"`
	IsAlive             bool    `json:"alive"`
	NearestEnemyDist    float64 `json:"nearest_enemy_dist"`
	NearestEnemyAngle   float64 `json:"nearest_enemy_angle"`
	NearestEnemyVisible bool    `json:"nearest_enemy_visible"`
	AimToEnemyOffset    float64 `json:"aim_enemy_offset"`
	TimeSinceVisible    float64 `json:"time_since_visible"`
	EnemiesVisible      int     `json:"enemies_visible"`
	CheatLabel          string  `json:"cheat_label"`
}

type KillEvent struct {
	Timestamp      int64   `json:"ts"`
	KillerID       string  `json:"killer_id"`
	VictimID       string  `json:"victim_id"`
	Distance       float64 `json:"distance"`
	ReactionTimeMs float64 `json:"reaction_time_ms"`
}

type InputState struct {
	Keys     KeyState   `json:"keys"`
	Mouse    MouseState `json:"mouse"`
	Shooting bool       `json:"shooting"`
	Cheats   CheatFlags `json:"cheats"`
}

type KeyState struct {
	W bool `json:"w"`
	A bool `json:"a"`
	S bool `json:"s"`
	D bool `json:"d"`
}

type MouseState struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CheatFlags struct {
	Aimbot     bool `json:"aimbot"`
	SpeedHack  bool `json:"speedhack"`
	WallHack   bool `json:"wallhack"`
	TriggerBot bool `json:"triggerbot"`
}

type GameStateMsg struct {
	Tick        int64           `json:"tick"`
	Players     []PlayerState   `json:"players"`
	Projectiles []Projectile    `json:"projectiles"`
	Kills       []KillFeedEntry `json:"kills,omitempty"`
	Walls       []Wall          `json:"walls,omitempty"`
}

type PlayerState struct {
	ID       string  `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Aim      float64 `json:"aim"`
	HP       int     `json:"hp"`
	Alive    bool    `json:"alive"`
	Shooting bool    `json:"shooting"`
}

type Projectile struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Angle float64 `json:"angle"`
}

type KillFeedEntry struct {
	Killer string `json:"killer"`
	Victim string `json:"victim"`
	Ts     int64  `json:"ts"`
}

type Wall struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}
