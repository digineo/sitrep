package model

import "time"

// Session is an admin login. It is stored under the hash of its token.
type Session struct {
	Provider    string    `json:"provider"`
	Subject     string    `json:"subject"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email,omitempty"`
	Expires     time.Time `json:"expires"`
}
