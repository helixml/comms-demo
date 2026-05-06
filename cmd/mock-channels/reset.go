package main

import (
	"flag"
	"fmt"

	"github.com/helixml/comms-demo/store"
)

func runReset(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	dbPath := fs.String("db", envOr("MOCK_CHANNELS_DB", "mock-channels.db"), "sqlite db path")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("reset: parse flags: %w", err)
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := st.Reset(); err != nil {
		return err
	}
	fmt.Println("reset: all channels and messages wiped")
	return nil
}
