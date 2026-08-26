package domain

import "time"

type Event struct {
	Sequence int64          `json:"sequence"`
	Type     string         `json:"type"`
	Actor    string         `json:"actor"`
	At       time.Time      `json:"at"`
	Details  map[string]any `json:"details,omitempty"`
}

func (c *ReleaseCase) record(kind, actor string, at time.Time, details map[string]any) {
	c.EventSequence++
	c.Events = append(c.Events, Event{Sequence: c.EventSequence, Type: kind, Actor: actor, At: UTC(at), Details: details})
}
