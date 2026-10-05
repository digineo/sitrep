package model

import "time"

// Session is an admin login. It is stored under the hash of its token.
type Session struct {
	Account string    `json:"account"`
	Expires time.Time `json:"expires"`
}
