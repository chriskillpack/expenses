package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	expenses "github.com/chriskillpack/expense-tracker"
	"github.com/plaid/plaid-go/v12/plaid"
)

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

type appConfig struct {
	DbFile            string      `toml:"db_file"`
	PlaidEnvironment  Environment `toml:"plaid_environment"`
	PlaidClientId     EnvVar      `toml:"plaid_client_id"`
	PlaidClientSecret EnvVar      `toml:"plaid_client_secret"`
	ServerPort        int         `toml:"server_port"`
	TLSCertFile       string      `toml:"https_cert_file"`
	TLSKeyFile        string      `toml:"https_key_file"`
}

var configFile = flag.String("config", "config.toml", "Path to configuration file")

func main() {
	flag.Parse()

	appConfig := appConfig{}
	_, err := toml.DecodeFile(*configFile, &appConfig)
	if err != nil {
		log.Fatal(err)
	}

	config := plaid.NewConfiguration()
	config.UseEnvironment(plaid.Environment(appConfig.PlaidEnvironment))
	config.AddDefaultHeader("PLAID-CLIENT-ID", string(appConfig.PlaidClientId))
	config.AddDefaultHeader("PLAID-SECRET", string(appConfig.PlaidClientSecret))

	log.Print("Opening DB")
	db, err := expenses.NewDB(appConfig.DbFile)
	if err != nil {
		log.Fatal(err)
	}

	srv := expenses.NewServer(
		plaid.NewAPIClient(config),
		db,
		appConfig.ServerPort,
		appConfig.TLSCertFile,
		appConfig.TLSKeyFile)

	// Start up the HTTPS server
	log.Print("Server starting")
	srv.Start()
}
