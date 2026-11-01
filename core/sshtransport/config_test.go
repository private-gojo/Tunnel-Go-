package sshtransport

import "testing"

func TestConfigRequiresHostKeyPolicy(t *testing.T) {
	_, err := (Config{
		Address: "127.0.0.1:22",
		Username: "test",
		Password: "secret",
	}).clientConfig()
	if err == nil {
		t.Fatal("expected host key policy to be required")
	}
}

func TestConfigRequiresAuthentication(t *testing.T) {
	_, err := (Config{
		Address: "127.0.0.1:22",
		Username: "test",
		InsecureSkipHostKeyVerification: true,
	}).clientConfig()
	if err == nil {
		t.Fatal("expected authentication method to be required")
	}
}

func TestConfigAllowsExplicitInsecureMode(t *testing.T) {
	cfg, err := (Config{
		Address: "127.0.0.1:22",
		Username: "test",
		Password: "secret",
		InsecureSkipHostKeyVerification: true,
	}).clientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostKeyCallback == nil {
		t.Fatal("host key callback must be configured")
	}
}
