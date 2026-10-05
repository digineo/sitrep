package main

import (
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/store"
)

// grantOwner makes the account named by login an owner of the instance,
// creating it if needed. It is the way back in when no owner can sign in.
func grantOwner(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "grant-owner takes one argument: an email address, or a username for basic auth")
		return 2
	}

	_, cfg, provider, ok := loadConfig(stderr)
	if !ok {
		return 1
	}

	log, err := newLogger(cfg, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	login, byEmail := args[0], true
	if dir, ok := provider.(auth.Directory); ok {
		users, err := dir.Users()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if !slices.Contains(users, login) {
			fmt.Fprintf(stderr, "the %s provider has no user %q\n", cfg.Auth, login)
			return 1
		}
		byEmail = false
	} else if login, ok = auth.ParseEmail(login); !ok {
		fmt.Fprintf(stderr, "%q is not an email address\n", args[0])
		return 1
	}

	db, err := store.Open(cfg.DB)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = db.Close() }()

	var old model.Role
	acc, err := db.Grant(cfg.Auth, login, byEmail, func(a *model.Account) error {
		old = a.Role
		a.Role = model.RoleOwner
		return nil
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	log.Info("made an account an owner from the command line",
		slog.String("account", acc.ID),
		slog.String("from", string(old)))
	if acc.Subject == "" {
		fmt.Fprintf(stdout, "%s is an owner from their next sign-in with this email address, if the identity provider verifies it\n", login)
	} else {
		fmt.Fprintf(stdout, "%s is an owner\n", login)
	}
	return 0
}
