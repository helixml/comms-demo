package server

import (
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/helixml/comms-demo/domain"
	"github.com/helixml/comms-demo/store"
)

type indexView struct {
	Email *domain.Channel
	Slack *domain.Channel
	SMS   *domain.Channel
}

type channelInboxView struct {
	Channel domain.Channel
	Threads []store.ThreadSummary
	Frame   bool
}

type threadView struct {
	Channel domain.Channel
	Thread  store.ThreadSummary
	Frame   bool
}

type composeView struct {
	Channel domain.Channel
	Frame   bool
}

type adminView struct {
	Channels []domain.Channel
}

// boundChannels groups configured channels by kind for the index page.
type boundChannels struct {
	Email *domain.Channel
	Slack *domain.Channel
	SMS   *domain.Channel
}

func bindChannels(in []domain.Channel) boundChannels {
	out := boundChannels{}
	for i := range in {
		c := in[i]
		switch c.Kind {
		case domain.KindEmail:
			if out.Email == nil {
				out.Email = &c
			}
		case domain.KindSlack:
			if out.Slack == nil {
				out.Slack = &c
			}
		case domain.KindSMS:
			if out.SMS == nil {
				out.SMS = &c
			}
		}
	}
	return out
}

// templateForKind returns the channel-kind-specific template name for a view.
func templateForKind(kind domain.ChannelKind, view string) string {
	return string(kind) + "_" + view + ".html"
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func splitRecipients(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func decodeMessage(r *http.Request) (*domain.Message, error) {
	defer func() { _ = r.Body.Close() }()
	limited := http.MaxBytesReader(nil, r.Body, 4*1024*1024)
	var m domain.Message
	if err := json.NewDecoder(limited).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func parseTemplates(rootFS fs.FS) (*template.Template, error) {
	sub, err := fs.Sub(rootFS, "templates")
	if err != nil {
		return nil, err
	}
	funcs := template.FuncMap{
		"fmtTime": func(unix int64) string {
			if unix == 0 {
				return ""
			}
			return time.Unix(unix, 0).Format("2006-01-02 15:04")
		},
		"fmtClock": func(unix int64) string {
			if unix == 0 {
				return ""
			}
			return time.Unix(unix, 0).Format("15:04")
		},
		"fmtDay": func(unix int64) string {
			if unix == 0 {
				return ""
			}
			return time.Unix(unix, 0).Format("Mon 02 Jan")
		},
		"toCSV": func(s []string) string { return strings.Join(s, ", ") },
		"first": func(s []string) string {
			if len(s) == 0 {
				return ""
			}
			return s[0]
		},
		"truncate": func(s string, n int) string {
			s = strings.TrimSpace(s)
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
		"initials": func(s string) string {
			s = strings.TrimSpace(s)
			if s == "" {
				return "?"
			}
			// Phone numbers: render the last two digits so each contact is distinct.
			if strings.HasPrefix(s, "+") {
				digits := strings.Map(func(r rune) rune {
					if r >= '0' && r <= '9' {
						return r
					}
					return -1
				}, s)
				if len(digits) >= 2 {
					return digits[len(digits)-2:]
				}
				if digits != "" {
					return digits
				}
			}
			parts := strings.FieldsFunc(s, func(r rune) bool {
				return r == ' ' || r == '@' || r == '.' || r == '-' || r == '_'
			})
			if len(parts) == 0 {
				return strings.ToUpper(s[:1])
			}
			out := strings.ToUpper(parts[0][:1])
			if len(parts) > 1 && parts[1] != "" {
				out += strings.ToUpper(parts[1][:1])
			}
			return out
		},
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"lastMsg": func(msgs []domain.StoredMessage) domain.StoredMessage {
			if len(msgs) == 0 {
				return domain.StoredMessage{}
			}
			return msgs[len(msgs)-1]
		},
		"otherParty": func(msgs []domain.StoredMessage, localIdentity string) string {
			for _, m := range msgs {
				if m.From != "" && m.From != localIdentity {
					return m.From
				}
				for to := range strings.SplitSeq(m.ToCSV, ",") {
					to = strings.TrimSpace(to)
					if to != "" && to != localIdentity {
						return to
					}
				}
			}
			return ""
		},
		"isOutbound": func(d domain.Direction) bool { return d == domain.DirectionOutbound },
		"isHTML":     func(s string) bool { return strings.EqualFold(s, "text/html") },
		"safeHTML":   func(s string) template.HTML { return template.HTML(s) }, //nolint:gosec // sandboxed by iframe in template
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("dict: odd number of arguments")
			}
			m := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				k, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict: keys must be strings")
				}
				m[k] = values[i+1]
			}
			return m, nil
		},
	}

	t := template.New("").Funcs(funcs)
	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".html") {
			return nil
		}
		b, rerr := fs.ReadFile(sub, path)
		if rerr != nil {
			return rerr
		}
		_, perr := t.New(path).Parse(string(b))
		return perr
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}
