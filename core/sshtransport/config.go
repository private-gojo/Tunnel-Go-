package sshtransport

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Config struct {
	Address                       string
	Username                      string
	Password                      string
	PrivateKey                    []byte
	PrivateKeyPassphrase          []byte
	KnownHostsPath                string
	InsecureSkipHostKeyVerification bool
	HandshakeTimeoutSeconds       int
}

func (c Config) clientConfig() (*ssh.ClientConfig, error) {
	if c.Username == "" {
		return nil, fmt.Errorf("SSH username is required")
	}

	auth := make([]ssh.AuthMethod, 0, 2)
	if c.Password != "" {
		auth = append(auth, ssh.Password(c.Password))
	}
	if len(c.PrivateKey) > 0 {
		var signer ssh.Signer
		var err error
		if len(c.PrivateKeyPassphrase) > 0 {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(c.PrivateKey, c.PrivateKeyPassphrase)
		} else {
			signer, err = ssh.ParsePrivateKey(c.PrivateKey)
		}
		if err != nil {
			return nil, fmt.Errorf("parse SSH private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("SSH password or private key is required")
	}

	var hostKeyCallback ssh.HostKeyCallback
	switch {
	case c.InsecureSkipHostKeyVerification:
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	case c.KnownHostsPath != "":
		if _, err := os.Stat(c.KnownHostsPath); err != nil {
			return nil, fmt.Errorf("known_hosts file: %w", err)
		}
		callback, err := knownhosts.New(c.KnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("load known_hosts: %w", err)
		}
		hostKeyCallback = callback
	default:
		return nil, fmt.Errorf("SSH host key verification requires known_hosts or explicit insecure mode")
	}

	return &ssh.ClientConfig{
		User:            c.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}, nil
}
