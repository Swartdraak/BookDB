package auth

import "testing"

func TestResolveMode(t *testing.T) {
	cases := []struct {
		name     string
		settings Settings
		want     Mode
	}{
		{name: "disabled", settings: Settings{}, want: ModeDisabled},
		{name: "local", settings: Settings{LocalEnabled: true}, want: ModeLocal},
		{name: "oidc", settings: Settings{OIDC: OIDCSettings{Enabled: true}}, want: ModeOIDC},
		{name: "hybrid", settings: Settings{LocalEnabled: true, OIDC: OIDCSettings{Enabled: true}}, want: ModeHybrid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveMode(tc.settings); got != tc.want {
				t.Fatalf("ResolveMode()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestValidateSettings(t *testing.T) {
	if err := ValidateSettings(Settings{}); err == nil {
		t.Fatal("expected disabled-mode validation error")
	}

	if err := ValidateSettings(Settings{LocalEnabled: true}); err != nil {
		t.Fatalf("local mode should validate: %v", err)
	}

	oidc := Settings{
		OIDC: OIDCSettings{Enabled: true, Issuer: "https://issuer", ClientID: "bookdb", ClientSecretRef: "vault://oidc/secret"},
	}
	if err := ValidateSettings(oidc); err != nil {
		t.Fatalf("oidc mode should validate: %v", err)
	}

	badOIDC := Settings{OIDC: OIDCSettings{Enabled: true, Issuer: "https://issuer", ClientID: "bookdb"}}
	if err := ValidateSettings(badOIDC); err == nil {
		t.Fatal("expected oidc secret/secret_ref validation error")
	}
}

func TestPublicOIDC(t *testing.T) {
	pub := PublicOIDC(Settings{})
	if pub.Enabled {
		t.Fatal("expected disabled public OIDC by default")
	}

	pub = PublicOIDC(Settings{OIDC: OIDCSettings{Enabled: true, Issuer: "https://issuer", ClientID: "bookdb", Audience: "bookdb-api", Discovery: true}})
	if !pub.Enabled || pub.Issuer != "https://issuer" || pub.ClientID != "bookdb" {
		t.Fatalf("unexpected OIDC public payload: %#v", pub)
	}
}
