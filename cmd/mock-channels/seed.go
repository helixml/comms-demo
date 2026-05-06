package main

import (
	"flag"
	"fmt"

	"github.com/helixml/comms-demo/domain"
	"github.com/helixml/comms-demo/store"
)

func runSeed(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	dbPath := fs.String("db", envOr("MOCK_CHANNELS_DB", "mock-channels.db"), "sqlite db path")
	helixBase := fs.String("helix-base", envOr("HELIX_BASE_URL", "http://localhost:8080"), "Helix Org base URL")
	emailStream := fs.String("email-stream", "", "Helix stream id for email channel")
	slackStream := fs.String("slack-stream", "", "Helix stream id for slack channel")
	smsStream := fs.String("sms-stream", "", "Helix stream id for sms channel")
	emailIdent := fs.String("email-from", "alice@example.com", "local identity for email channel")
	slackIdent := fs.String("slack-from", "U0ALICE", "local identity for slack channel")
	smsIdent := fs.String("sms-from", "+15550100", "local identity for sms channel")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("seed: parse flags: %w", err)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	type seed struct {
		c domain.Channel
	}
	rows := []seed{
		{c: domain.Channel{
			ChannelID:     "email-main",
			Kind:          domain.KindEmail,
			DisplayName:   "Inbox",
			HelixStreamID: *emailStream,
			HelixBaseURL:  *helixBase,
			LocalIdentity: *emailIdent,
		}},
		{c: domain.Channel{
			ChannelID:     "slack-general",
			Kind:          domain.KindSlack,
			DisplayName:   "general",
			HelixStreamID: *slackStream,
			HelixBaseURL:  *helixBase,
			LocalIdentity: *slackIdent,
		}},
		{c: domain.Channel{
			ChannelID:     "sms-main",
			Kind:          domain.KindSMS,
			DisplayName:   "Messages",
			HelixStreamID: *smsStream,
			HelixBaseURL:  *helixBase,
			LocalIdentity: *smsIdent,
		}},
	}
	for _, r := range rows {
		c := r.c
		if err := st.UpsertChannel(&c); err != nil {
			return fmt.Errorf("seed channel %s: %w", c.ChannelID, err)
		}
		fmt.Printf("seeded %s (%s) → %s/webhooks/%s\n", c.ChannelID, c.Kind, c.HelixBaseURL, c.HelixStreamID)
	}
	return nil
}
