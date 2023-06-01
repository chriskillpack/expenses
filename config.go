package expenses

import (
	"fmt"
	"os"

	"github.com/plaid/plaid-go/v12/plaid"
)

// Environment is a string representation of a Plaid environment
type Environment string

func (e *Environment) UnmarshalText(text []byte) error {
	switch string(text) {
	case "development":
		*e = Environment(plaid.Development)
	case "sandbox":
		*e = Environment(plaid.Sandbox)
	case "production":
		*e = Environment(plaid.Production)
	default:
		return fmt.Errorf("unrecognized environment %q", string(text))
	}

	return nil
}

// EnvVar takes the value of an environment variable if its config value is
// !CONFIG_VAR_TO_LOOKUP
type EnvVar string

func (e *EnvVar) UnmarshalText(text []byte) error {
	s := string(text)

	if s[0] == '!' {
		env := s[1:]

		// The value is actually the name of an env var
		if val, ok := os.LookupEnv(env); ok && val != "" {
			*e = EnvVar(val)
			return nil
		} else {
			return fmt.Errorf("env var %s undefined or empty", env)
		}
	}

	*e = EnvVar(s)
	return nil
}

// AppConfig holds application config parsed from TOML config file
type AppConfig struct {
	DbFile            string      `toml:"db_file"`
	PlaidEnvironment  Environment `toml:"plaid_environment"`
	PlaidClientId     EnvVar      `toml:"plaid_client_id"`
	PlaidClientSecret EnvVar      `toml:"plaid_client_secret"`
	ServerPort        int         `toml:"server_port"`
	TLSCertFile       string      `toml:"https_cert_file"`
	TLSKeyFile        string      `toml:"https_key_file"`
}
