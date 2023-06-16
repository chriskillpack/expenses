package expenses

import (
	"fmt"
	"os"
)

// AppConfig holds application config parsed from TOML config file
type AppConfig struct {
	DbFile            string `toml:"db_file"`
	Environment       string `toml:"environment"`
	EnvFile           string `toml:"env_file"`
	PlaidClientId     string `toml:"plaid_client_id"`
	PlaidClientSecret string `toml:"plaid_client_secret"`
	ServerPort        int    `toml:"server_port"`
	TLSCertFile       string `toml:"https_cert_file"`
	TLSKeyFile        string `toml:"https_key_file"`
}

func resolve(wtf string) (string, error) {
	if len(wtf) == 0 {
		return "", nil
	}

	if wtf[0] == '!' {
		env := wtf[1:]
		if val, ok := os.LookupEnv(env); ok && val != "" {
			return val, nil
		} else {
			return "", fmt.Errorf("env var %s undefined or empty", env)
		}
	}
	return "", nil
}

func (c *AppConfig) ResolveEnvVars() error {
	if pcid, err := resolve(string(c.PlaidClientId)); err != nil {
		return err
	} else {
		c.PlaidClientId = pcid
	}

	if pcs, err := resolve(string(c.PlaidClientSecret)); err != nil {
		return err
	} else {
		c.PlaidClientSecret = pcs
	}

	return nil
}
