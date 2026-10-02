package basic

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Parameters of hashes created by Hash: RFC 9106's second recommended option.
const (
	hashMemory  = 64 * 1024
	hashTime    = 3
	hashThreads = 4
)

// Hash returns an argon2id hash of password as PHC string.
func Hash(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	key := argon2.IDKey(
		[]byte(password),
		salt,
		hashTime,
		hashMemory,
		hashThreads,
		32,
	)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		hashMemory,
		hashTime,
		hashThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// Verify reports whether password matches a bcrypt or argon2id hash.
func Verify(hash, password string) bool {
	if strings.HasPrefix(hash, "$argon2id$") {
		h, err := parseArgon2id(hash)
		if err != nil {
			return false
		}

		key := argon2.IDKey(
			[]byte(password),
			h.salt,
			h.time,
			h.memory,
			h.threads,
			uint32(len(h.key)),
		)
		return subtle.ConstantTimeCompare(key, h.key) == 1
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// checkHash validates a hash from the users file. Its errors never contain
// the hash.
func checkHash(hash string) error {
	switch {
	case strings.HasPrefix(hash, "$2a$"),
		strings.HasPrefix(hash, "$2b$"),
		strings.HasPrefix(hash, "$2y$"):
		cost, err := bcrypt.Cost([]byte(hash))
		if err != nil || len(hash) != 60 {
			return errors.New("malformed bcrypt hash")
		}

		if cost < 10 {
			return errors.New("bcrypt cost must be at least 10")
		}
		return nil
	case strings.HasPrefix(hash, "$argon2id$"):
		_, err := parseArgon2id(hash)
		return err
	}
	return errors.New("unsupported hash format, use bcrypt or argon2id")
}

type argon2idHash struct {
	memory, time uint32
	threads      uint8
	salt, key    []byte
}

// parseArgon2id parses
// "$argon2id$v=19$m=<KiB>,t=<iterations>,p=<parallelism>$<salt>$<hash>".
// It rejects parameters below the OWASP minimum and above limits that keep
// a login from stalling the server.
func parseArgon2id(s string) (*argon2idHash, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, errors.New("malformed argon2id hash")
	}

	if parts[2] != "v=19" {
		return nil, errors.New("only argon2id version 19 is supported")
	}

	var m, t, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil ||
		parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", m, t, p) {
		return nil, errors.New("malformed argon2id parameters")
	}

	if m < 19456 || m > 1048576 || t < 2 || t > 10 || p < 1 || p > 16 {
		return nil, errors.New(
			"argon2id parameters must be within " +
				"m=19456..1048576, t=2..10, p=1..16",
		)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 {
		return nil, errors.New(
			"argon2id salt must be at least 16 bytes in unpadded base64",
		)
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 32 {
		return nil, errors.New(
			"argon2id hash must be at least 32 bytes in unpadded base64",
		)
	}
	return &argon2idHash{
		memory:  m,
		time:    t,
		threads: uint8(p),
		salt:    salt,
		key:     key,
	}, nil
}
