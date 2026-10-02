package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"

	"github.com/digineo/sitrep/internal/auth/basic"
)

// hashPassword prints an argon2id hash of a password for the users file.
// It prompts twice on a terminal, else it reads the first line of stdin.
func hashPassword(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hash-password", flag.ContinueOnError)
	fs.SetOutput(stderr)
	user := fs.String("user", "", `print "<name>:<hash>" for this user`)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "hash-password takes no arguments")
		return 2
	}

	invalid := func(r rune) bool { return r == ':' || unicode.IsSpace(r) }
	if *user != "" && strings.ContainsFunc(*user, invalid) {
		fmt.Fprintln(stderr, "the user name must not contain colons or whitespace")
		return 2
	}

	password, err := readPassword(stdin, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if password == "" {
		fmt.Fprintln(stderr, "the password must not be empty")
		return 1
	}

	hash := basic.Hash(password)
	if *user != "" {
		hash = *user + ":" + hash
	}

	fmt.Fprintln(stdout, hash)
	return 0
}

func readPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		read := func() ([]byte, error) { return term.ReadPassword(int(f.Fd())) }
		return promptPassword(read, stderr)
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

// promptPassword asks for the password twice, reading each entry without
// echo, and fails if they differ.
func promptPassword(read func() ([]byte, error), stderr io.Writer) (string, error) {
	fmt.Fprint(stderr, "Password: ")
	first, err := read()
	fmt.Fprint(stderr, "\nRepeat password: ")
	if err != nil {
		return "", err
	}

	second, err := read()
	fmt.Fprintln(stderr)
	if err != nil {
		return "", err
	}

	if string(first) != string(second) {
		return "", errors.New("the passwords do not match")
	}
	return string(first), nil
}
