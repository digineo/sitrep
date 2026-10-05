// Command sitrep serves multi-tenant status pages.
package main

import (
	"fmt"
	"io"
	"os"
	_ "time/tzdata" // time zone lookups never depend on the host
)

const usage = `Usage: sitrep <command> [flags]

Commands:
  serve            run the server, configured by SITREP_* environment variables
  hash-password    print an argon2id hash for the basic auth users file
                   flags: -user <name> prints "<name>:<hash>"
  healthcheck      exit with 0 if the server at SITREP_LISTEN is healthy
  help             show this help
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "serve":
		return serve(args[1:], stderr)
	case "hash-password":
		return hashPassword(args[1:], stdin, stdout, stderr)
	case "healthcheck":
		return healthcheck(args[1:], stderr)
	}

	fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
	return 2
}
