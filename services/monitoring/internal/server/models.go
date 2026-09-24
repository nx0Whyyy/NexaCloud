package server

import "time"

type Snapshot struct {
	ID        uint      `json:"-" gorm:"primaryKey"`
	Component string    `json:"component" gorm:"index;size:40;not null"`
	Status    string    `json:"status" gorm:"size:20;not null"`
	LatencyMS int64     `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at" gorm:"index;not null"`
}

type Incident struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Component   string     `json:"component" gorm:"index;size:40;not null"`
	Title       string     `json:"title" gorm:"size:180;not null"`
	Status      string     `json:"status" gorm:"index;size:20;not null"`
	StartedAt   time.Time  `json:"started_at" gorm:"index;not null"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	LastMessage string     `json:"message" gorm:"size:500"`
}

type Component struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	LatencyMS   int64     `json:"latency_ms"`
	Uptime24H   float64   `json:"uptime_24h"`
	CheckedAt   time.Time `json:"checked_at"`
	Error       string    `json:"error,omitempty"`
}

type StatusResponse struct {
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Components []Component `json:"components"`
	Incidents  []Incident  `json:"incidents"`
}
