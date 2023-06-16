package expenses

import (
	"fmt"
	"os"
)

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
	DbFile            string `toml:"db_file"`
	Environment       string `toml:"environment"`
	PlaidClientId     EnvVar `toml:"plaid_client_id"`
	PlaidClientSecret EnvVar `toml:"plaid_client_secret"`
	ServerPort        int    `toml:"server_port"`
	TLSCertFile       string `toml:"https_cert_file"`
	TLSKeyFile        string `toml:"https_key_file"`
}
