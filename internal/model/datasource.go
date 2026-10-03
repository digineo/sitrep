package model

import "time"

// DataSource is a configured backend that panels query. Config holds the
// plain settings; secret settings are kept apart, sealed.
type DataSource struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Config    map[string]string `json:"config"`
	Secrets   map[string][]byte `json:"secrets,omitempty"`
	Revision  int64             `json:"revision"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}
