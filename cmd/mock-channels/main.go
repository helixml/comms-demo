// mock-channels is the CLI entrypoint. Subcommands: serve, seed, reset.
package main

import (
	"fmt"
	"log/slog"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "serve":
		err = runServe(args)
	case "seed":
		err = runSeed(args)
	case "reset":
		err = runReset(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", cmd)
		usage()
		os.Exit(2)
	}

	if err != nil {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("fatal", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `mock-channels — Helix Org demo provider

Subcommands:
  serve   run the HTTP server
  seed    create the three default channels (email/slack/sms) pointing at a Helix base URL
  reset   wipe all channels and messages from the local DB

Examples:
  mock-channels serve --addr :7765 --db ./mock-channels.db
  mock-channels seed --helix-base http://localhost:8080 \
    --email-stream strm-email --slack-stream strm-slack --sms-stream strm-sms
  mock-channels reset --db ./mock-channels.db
`)
}
