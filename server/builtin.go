package server

import (
	"event-pool/secrets"
	"event-pool/secrets/local"
)

type ConsensusType string

// secretsManagerBackends defines the SecretManager factories for different
// secret management solutions
var secretsManagerBackends = map[secrets.SecretsManagerType]secrets.SecretsManagerFactory{
	secrets.Local: local.SecretsManagerFactory,
}

func ConsensusSupported(value string) bool {

	return true
}
