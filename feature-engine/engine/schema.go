package engine

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

type FeatureVector struct {
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

	AimDeltaMean1s         float64 `json:"aim_delta_mean_1s"`
	AimDeltaMean5s         float64 `json:"aim_delta_mean_5s"`
	AimDeltaMax1s          float64 `json:"aim_delta_max_1s"`
	AimSnapCount5s         int     `json:"aim_snap_count_5s"`
	AimToEnemyOffsetMean5s float64 `json:"aim_to_enemy_offset_mean_5s"`

	HitRate5s         float64 `json:"hit_rate_5s"`
	HitRate30s        float64 `json:"hit_rate_30s"`
	ShotsFired5s      int     `json:"shots_fired_5s"`
	KillsPer30s       int     `json:"kills_per_30s"`
	TimeToKillMean30s float64 `json:"time_to_kill_mean_30s"`

	SpeedMean1s            float64 `json:"speed_mean_1s"`
	SpeedMean5s            float64 `json:"speed_mean_5s"`
	SpeedMax1s             float64 `json:"speed_max_1s"`
	DirectionChangeCount5s int     `json:"direction_change_count_5s"`

	AimLockRatio5s       float64 `json:"aim_lock_ratio_5s"`
	PrefireRatio5s       float64 `json:"prefire_ratio_5s"`
	ReactionTimeMean5s   float64 `json:"reaction_time_mean_5s"`
	EnemyTrackingScore5s float64 `json:"enemy_tracking_score_5s"`
}

type AlertEvent struct {
	Timestamp  int64   `json:"ts"`
	PlayerID   string  `json:"player_id"`
	Source     string  `json:"source"`
	Rule       string  `json:"rule"`
	CheatType  string  `json:"cheat_type"`
	Confidence float64 `json:"confidence"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
}
