package auth

import (
	"fmt"
	"strings"
)

// Mode describes the enabled authentication capability set.
type Mode string

const (
	ModeDisabled Mode = "disabled"
	ModeLocal    Mode = "local"
	ModeOIDC     Mode = "oidc"
	ModeHybrid   Mode = "hybrid"
)

// Settings is the runtime auth config subset needed by the skeleton.
type Settings struct {
	LocalEnabled bool
	OIDC         OIDCSettings
}

// OIDCSettings is the runtime OIDC config subset used by the skeleton.
type OIDCSettings struct {
	Enabled         bool
	Issuer          string
	ClientID        string
	ClientSecret    string
	ClientSecretRef string
	Audience        string
	Discovery       bool
}

// OIDCPublic is safe to expose in diagnostics and runtime endpoints.
type OIDCPublic struct {
	Enabled   bool   `json:"enabled"`
	Issuer    string `json:"issuer,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Audience  string `json:"audience,omitempty"`
	Discovery bool   `json:"discovery"`
}

// ResolveMode determines the active auth mode.
func ResolveMode(settings Settings) Mode {
	hasLocal := settings.LocalEnabled
	hasOIDC := settings.OIDC.Enabled
	switch {
	case hasLocal && hasOIDC:
		return ModeHybrid
	case hasLocal:
		return ModeLocal
	case hasOIDC:
		return ModeOIDC
	default:
		return ModeDisabled
	}
}

// ValidateSettings applies M1 auth skeleton configuration rules.
func ValidateSettings(settings Settings) error {
	mode := ResolveMode(settings)
	if mode == ModeDisabled {
		return fmt.Errorf("config: at least one authentication mode must be enabled")
	}
	if !settings.OIDC.Enabled {
		return nil
	}
	if strings.TrimSpace(settings.OIDC.Issuer) == "" {
		return fmt.Errorf("config: auth.oidc.issuer is required when OIDC is enabled")
	}
	if strings.TrimSpace(settings.OIDC.ClientID) == "" {
		return fmt.Errorf("config: auth.oidc.client_id is required when OIDC is enabled")
	}
	if strings.TrimSpace(settings.OIDC.ClientSecret) == "" && strings.TrimSpace(settings.OIDC.ClientSecretRef) == "" {
		return fmt.Errorf("config: auth.oidc.client_secret or auth.oidc.client_secret_ref is required when OIDC is enabled")
	}
	return nil
}

// PublicOIDC returns the non-secret OIDC surface.
func PublicOIDC(settings Settings) OIDCPublic {
	if !settings.OIDC.Enabled {
		return OIDCPublic{Enabled: false}
	}
	return OIDCPublic{
		Enabled:   true,
		Issuer:    settings.OIDC.Issuer,
		ClientID:  settings.OIDC.ClientID,
		Audience:  settings.OIDC.Audience,
		Discovery: settings.OIDC.Discovery,
	}
}
