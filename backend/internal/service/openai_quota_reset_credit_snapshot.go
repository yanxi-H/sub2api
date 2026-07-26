package service

import (
	"encoding/json"
	"time"
)

// OpenAIResetCreditSnapshotExtraKey is the cached, sanitized reset-credit
// snapshot used by the admin dashboard. It never contains upstream credit IDs.
const OpenAIResetCreditSnapshotExtraKey = "openai_rate_limit_reset_credits"

// OpenAIResetCreditSnapshot is persisted in Account.Extra after a successful
// upstream quota query. CheckedAt distinguishes an unqueried account from one
// whose available credit count is genuinely zero.
type OpenAIResetCreditSnapshot struct {
	AvailableCount int                                `json:"available_count"`
	Credits        []OpenAIRateLimitResetCreditDetail `json:"credits,omitempty"`
	CheckedAt      time.Time                          `json:"checked_at"`
}

func newOpenAIResetCreditSnapshot(credits *OpenAIRateLimitResetCredits, checkedAt time.Time) *OpenAIResetCreditSnapshot {
	if credits == nil {
		return nil
	}
	count := credits.AvailableCount
	if count < 0 {
		count = 0
	}
	return &OpenAIResetCreditSnapshot{
		AvailableCount: count,
		Credits:        append([]OpenAIRateLimitResetCreditDetail(nil), credits.Credits...),
		CheckedAt:      checkedAt.UTC(),
	}
}

// OpenAIResetCreditSnapshotFromExtra decodes the tolerant map representation
// produced by the JSON extra column. Invalid or incomplete values are treated
// as absent so the UI never mistakes malformed data for zero credits.
func OpenAIResetCreditSnapshotFromExtra(extra map[string]any) *OpenAIResetCreditSnapshot {
	if len(extra) == 0 {
		return nil
	}
	raw, ok := extra[OpenAIResetCreditSnapshotExtraKey]
	if !ok || raw == nil {
		return nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var snapshot OpenAIResetCreditSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil || snapshot.AvailableCount < 0 || snapshot.CheckedAt.IsZero() {
		return nil
	}
	return &snapshot
}

// SupportsOpenAIResetCredits intentionally excludes shadow accounts: their
// credentials and reset credits belong to the parent account and must not be
// counted twice on the dashboard.
func SupportsOpenAIResetCredits(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth && !account.IsShadow()
}
