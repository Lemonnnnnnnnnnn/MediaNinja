// Package events defines transport-independent execution notifications.
package events

import "time"

type Event struct {
	Message   string    `json:"message,omitempty"`
	Type      string    `json:"type"`
	File      string    `json:"file,omitempty"`
	Completed int64     `json:"completed,omitempty"`
	Total     int64     `json:"total,omitempty"`
	Unit      string    `json:"unit,omitempty"`
	Error     string    `json:"error,omitempty"`
	Time      time.Time `json:"time"`
}

// Observer callbacks are serialized per crawler and should return promptly.
type Observer func(Event)
