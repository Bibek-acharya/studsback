package coins

import (
	"context"
	"encoding/json"
	"time"
)

// Config version history — the readable half of 04 §6's "a config version history".
//
// The writes have existed since the Service was built; every pricing change has
// appended a row. What was missing is anything to read them back, which is the only
// half a history page can use. So this is a READ path over an audit table that was
// already being maintained — no new state, no new migration.

// ConfigVersionHistoryDefault is how many entries a request with no limit gets.
//
// 50, because the question an admin opens this page for is "what did I just change,
// and what did I change before that" — a handful of recent changes — and 50 covers
// more than that at the current rate of change without asking a table to return its
// whole contents.
const ConfigVersionHistoryDefault = 50

// ConfigVersionHistoryMax bounds the response.
//
// The bound exists for the same reason as HealthMaxWindow: this route is reachable by
// anyone who can rewrite coin pricing, and `?limit=100000` should not return every
// pricing change in the platform's history.
const ConfigVersionHistoryMax = 200

// ConfigVersionEntry is one row of the history, as returned.
//
// It is a DTO rather than ConfigVersion itself for one reason: the raw model carries
// `previous_json` and `new_json` as TEXT, and the console needs them decoded into
// configs so it can render a field-level diff. Decoding here rather than in the
// frontend also means a row written by an older build — or a hand-edited one — is
// handled in one place.
type ConfigVersionEntry struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ChangedBy string    `json:"changed_by"`
	// ChangedByUserID is kept alongside the name so the console can link the actor
	// without parsing the "admin:<id>" convention, which is a display format rather
	// than a storage contract.
	ChangedByUserID uint `json:"changed_by_user_id"`
	// Previous and New are the decoded snapshots. Either may be nil on the FIRST
	// version — there is no previous — and on a row whose stored text did not parse.
	Previous *EconomyConfig `json:"previous"`
	New      *EconomyConfig `json:"new"`
	// PreviousJSON and NewJSON are the raw stored text, ALWAYS present.
	//
	// They are not redundant with the decoded fields: they are what makes an
	// undecodable row repairable rather than merely skipped. A row whose payload does
	// not parse would otherwise be a dead entry with no way to see what it said.
	PreviousJSON string `json:"previous_json"`
	NewJSON      string `json:"new_json"`
}

// ConfigVersionHistory returns the most recent config versions, newest first.
//
// The default limit is applied when limit is zero or negative, and capped when it is
// larger than ConfigVersionHistoryMax. Both clamps live here rather than in the
// handler so there is one place that decides how many rows a caller can ask for.
func (s *Service) ConfigVersionHistory(ctx context.Context, limit int) ([]ConfigVersionEntry, error) {
	if s == nil || s.versions == nil {
		return nil, ErrNoDatabase
	}
	if limit <= 0 {
		limit = ConfigVersionHistoryDefault
	}
	if limit > ConfigVersionHistoryMax {
		limit = ConfigVersionHistoryMax
	}

	rows, err := s.versions.RecentConfigVersions(limit)
	if err != nil {
		// An error, never an empty slice: "you have never changed a price" and "we
		// could not read the audit trail" are opposite messages, and an admin who
		// reads the second as the first has been told their pricing is unchanged when
		// nobody knows what it is.
		return nil, err
	}

	out := make([]ConfigVersionEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ConfigVersionEntry{
			ID: row.ID, CreatedAt: row.CreatedAt,
			ChangedBy: row.ChangedBy, ChangedByUserID: row.ChangedByUserID,
			PreviousJSON: row.PreviousJSON, NewJSON: row.NewJSON,
			Previous: decodeConfigSnapshot(row.PreviousJSON),
			New:      decodeConfigSnapshot(row.NewJSON),
		})
	}
	return out, nil
}

// decodeConfigSnapshot parses one stored snapshot.
//
// A parse failure is nil, NOT an error, and that is the whole design: the row still
// appears in the list, with its raw text, and only the decoded pair is absent. One
// truncated payload — a half-finished write, a hand-edited row, a build that
// serialised differently — should cost that one entry its diff, not cost the admin the
// page. The alternative — failing the whole read — turns a cosmetic defect into an
// outage of the audit trail.
//
// An empty string is also nil, and that is the normal state of PreviousJSON on the
// first version ever written, so it must not be treated as corruption.
func decodeConfigSnapshot(raw string) *EconomyConfig {
	if raw == "" {
		return nil
	}
	var cfg EconomyConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil
	}
	return &cfg
}
