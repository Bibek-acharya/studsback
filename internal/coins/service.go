package coins

import (
	"encoding/json"
	"fmt"
)

// Service owns the economy configuration: load it through the cache, and
// write it behind validation plus an audit row. It also owns the entitlement
// domain, so that a gate has one object to call rather than two.
//
// repo is nil in a Service built by NewService, which is the admin-config-only
// wiring. Every entitlement method checks and returns ErrNoDatabase rather than
// panicking; see unlock.go's requireRepo. Use NewServiceWithRepository to get a
// Service that can reach the database.
type Service struct {
	config   *ConfigStore
	versions VersionStore
	repo     *Repository
}

func NewService(config *ConfigStore, versions VersionStore) *Service {
	return &Service{config: config, versions: versions}
}

// GetEconomyConfig returns the current economy config, from the cache when it
// is fresh. This is the read the admin screen renders and the read the ledger
// will make on every unlock, so it must never fail for a reason the caller
// can act on.
func (s *Service) GetEconomyConfig() (EconomyConfig, error) {
	return s.config.Load()
}

// UpdateEconomyConfig merges a partial request onto the current config,
// validates the whole merged result, persists it, and records the change.
//
// Ordering is the contract:
//
//  1. load the current config (merge base);
//  2. merge and validate — nothing is written when validation fails, so a bad
//     price never reaches storage and never lands the config halfway applied;
//  3. persist and invalidate the cache, so the next read sees the new value;
//  4. append the version row.
//
// Step 4 is a second statement rather than part of the same transaction:
// system.Repository.SetSystemSetting takes its own *gorm.DB and exposes no way
// to enlist in a caller's transaction, and internal/system is not ours to
// change. So if the version insert fails after the setting is stored, this
// returns an error while the new config is already live — the cache has
// already been invalidated, so readers are consistent, but the audit row is
// missing. An admin who sees that failure should GET the config to see what
// actually took effect rather than blindly retrying. Steps 1-3 are the
// user-visible outcome; step 4 is the record of it.
func (s *Service) UpdateEconomyConfig(req UpdateEconomyConfigRequest, actorUserID uint) (EconomyConfig, error) {
	current, err := s.config.Load()
	if err != nil {
		return EconomyConfig{}, err
	}

	next := applyEconomyConfigUpdate(current, req)
	if err := ValidateEconomyConfig(next); err != nil {
		return EconomyConfig{}, err
	}

	encoded, err := json.Marshal(next)
	if err != nil {
		return EconomyConfig{}, err
	}
	previous, err := json.Marshal(current)
	if err != nil {
		return EconomyConfig{}, err
	}

	if err := s.config.Save(string(encoded)); err != nil {
		return EconomyConfig{}, err
	}

	if err := s.versions.AppendConfigVersion(&ConfigVersion{
		PreviousJSON:    string(previous),
		NewJSON:         string(encoded),
		ChangedByUserID: actorUserID,
		ChangedBy:       actorLabel(actorUserID),
	}); err != nil {
		return EconomyConfig{}, fmt.Errorf("config saved but version history append failed: %w", err)
	}

	return next, nil
}
