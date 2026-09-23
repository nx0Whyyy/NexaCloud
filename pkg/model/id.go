package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func NewID() uuid.UUID {
	return uuid.New()
}

func MustParseID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

func JSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func Now() time.Time {
	return time.Now().UTC()
}
