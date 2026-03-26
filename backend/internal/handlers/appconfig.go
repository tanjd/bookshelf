package handlers

import (
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/tanjd/bookshelf/internal/models"
)

// appConfigFile is the YAML structure used for import/export of app settings.
type appConfigFile struct {
	AllowRegistration          string `yaml:"allow_registration,omitempty"`
	MaxCopiesPerUser           string `yaml:"max_copies_per_user,omitempty"`
	MaxActiveLoans             string `yaml:"max_active_loans,omitempty"`
	RequireVerifiedToBorrow    string `yaml:"require_verified_to_borrow,omitempty"`
	VerificationRequiresPhone  string `yaml:"verification_requires_phone,omitempty"`
	VerificationMinBooksShared string `yaml:"verification_min_books_shared,omitempty"`
	CoverRefreshInterval       string `yaml:"cover_refresh_interval,omitempty"`
}

var knownYAMLKeys = map[string]struct{}{
	"allow_registration":            {},
	"max_copies_per_user":           {},
	"max_active_loans":              {},
	"require_verified_to_borrow":    {},
	"verification_requires_phone":   {},
	"verification_min_books_shared": {},
	"cover_refresh_interval":        {},
}

// LoadYAMLConfig parses a bookshelf.yaml file and returns a flat key→value map
// of recognized settings. Unknown keys are logged as warnings and ignored.
// Returns nil map (not an error) when the file does not exist.
func LoadYAMLConfig(path string) (map[string]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Detect unknown keys before struct unmarshalling silently discards them.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err == nil {
		for k := range raw {
			if _, ok := knownYAMLKeys[k]; !ok {
				log.Warn().Str("key", k).Str("path", path).Msg("bookshelf.yaml: unknown setting key ignored")
			}
		}
	}

	var cfg appConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	kv := make(map[string]string)
	if cfg.AllowRegistration != "" {
		kv["allow_registration"] = cfg.AllowRegistration
	}
	if cfg.MaxCopiesPerUser != "" {
		kv["max_copies_per_user"] = cfg.MaxCopiesPerUser
	}
	if cfg.MaxActiveLoans != "" {
		kv["max_active_loans"] = cfg.MaxActiveLoans
	}
	if cfg.RequireVerifiedToBorrow != "" {
		kv["require_verified_to_borrow"] = cfg.RequireVerifiedToBorrow
	}
	if cfg.VerificationRequiresPhone != "" {
		kv["verification_requires_phone"] = cfg.VerificationRequiresPhone
	}
	if cfg.VerificationMinBooksShared != "" {
		kv["verification_min_books_shared"] = cfg.VerificationMinBooksShared
	}
	if cfg.CoverRefreshInterval != "" {
		kv["cover_refresh_interval"] = cfg.CoverRefreshInterval
	}
	return kv, nil
}

// settingsToYAML serialises a slice of AppSetting into YAML bytes suitable for
// use as a bookshelf.yaml config file.
func settingsToYAML(settings []models.AppSetting) ([]byte, error) {
	m := make(map[string]string, len(settings))
	for _, s := range settings {
		m[s.Key] = s.Value
	}

	cfg := appConfigFile{
		AllowRegistration:          m["allow_registration"],
		MaxCopiesPerUser:           m["max_copies_per_user"],
		MaxActiveLoans:             m["max_active_loans"],
		RequireVerifiedToBorrow:    m["require_verified_to_borrow"],
		VerificationRequiresPhone:  m["verification_requires_phone"],
		VerificationMinBooksShared: m["verification_min_books_shared"],
		CoverRefreshInterval:       m["cover_refresh_interval"],
	}
	return yaml.Marshal(cfg)
}
