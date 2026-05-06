# comms-demo (mock-channels)

A standalone Go webapp that pretends to be three messaging providers — **email**, **Slack**, and **SMS** — so Helix Org can be demoed end-to-end without hitting real third-party services.

It is **not** a real provider integration. There is **no auth, no signing, no replay protection**. Public demo.

## Wire spec

Helix Org's webhook transport is the contract. Two directions:

- **Helix Org → mock app** — Helix POSTs `application/json` with a `domain.Message` body to `POST /in/<channel_id>`. We respond `204`.
- **Mock app → Helix Org** — operator clicks Reply / Compose; we POST the same `Message` envelope to `<helix_base_url>/webhooks/<helix_stream_id>`.

Threading discipline (mirrors a real provider):

- First message in a thread: `thread_id == message_id`.
- Reply: copy `thread_id` from parent; `in_reply_to = parent.message_id`.

## What a calling tool must implement

To integrate with mock-channels, the calling service needs **one outbound call** and **one inbound endpoint**.

### 1. Send messages to mock-channels (outbound)

`POST http://<mock-host>:7765/in/<channel_id>`

- `Content-Type: application/json`
- Body is the `Message` envelope (see schema below).
- Returns `204 No Content` on success, `4xx` on validation failure.
- `<channel_id>` must already exist (seeded via CLI or `/admin`).

### 2. Receive messages from mock-channels (inbound)

Expose `POST /webhooks/<stream_id>` on your service. Mock-channels POSTs replies and operator-composed messages here.

- `Content-Type: application/json`
- Body is the same `Message` envelope.
- Respond `2xx` to acknowledge; non-2xx is logged as a send error and surfaced in the UI.
- The `<stream_id>` you advertise must match `helix_stream_id` registered for the channel in mock-channels.

### Message JSON schema

```json
{
  "from": "alice@example.com",
  "to": ["bob@example.com"],
  "subject": "Re: hello",
  "body": "the message body",
  "body_content_type": "text/plain",
  "thread_id": "abc123",
  "in_reply_to": "parent-msg-id",
  "message_id": "this-msg-id",
  "attachments": [
    { "filename": "x.pdf", "content_type": "application/pdf", "url": "https://...", "size_bytes": 1234 }
  ],
  "extra": { "any": "json" }
}
```

Required: `body`. All other fields are optional but follow the threading rules above.

### Identity conventions

- **email**: RFC 5322 addresses (`alice@example.com`)
- **slack**: user IDs (`U01ABC2DE`)
- **sms**: E.164 phone numbers (`+15551234567`)
- Empty `from` means a system-originated message with no human author.

## Run

```bash
make build
./bin/mock-channels serve --addr :7765 --db ./mock-channels.db
```

Open <http://localhost:7765/> for the three-pane phone view. <http://localhost:7765/admin> to add or edit channels.

Seed defaults (point at a Helix Org base URL):

```bash
./bin/mock-channels seed \
  --helix-base http://localhost:8080 \
  --email-stream strm-email \
  --slack-stream strm-slack \
  --sms-stream  strm-sms
```

Reset:

```bash
./bin/mock-channels reset
```

## Demo flow

1. Run `helix-org bootstrap` and `helix-org serve`.
2. Run `mock-channels serve` on a host Helix can reach.
3. Create three webhook Streams in Helix Org with `outbound_url` pointing at `http(s)://<mock-host>/in/<channel_id>` for each kind.
4. Run `mock-channels seed` (or use `/admin`) to register the matching `(channel_id, helix_stream_id)` rows.
5. A Helix Worker publishes a message → mock-channels receives it → renders in the appropriate channel UI.
6. Operator clicks Reply → mock-channels POSTs to `/webhooks/<streamID>` → Helix Org appends the event.
