package main

import (
	"flag"
	"log"

	"github.com/BurntSushi/toml"
	expenses "github.com/chriskillpack/expense-tracker"
	"github.com/joho/godotenv"
	"github.com/plaid/plaid-go/v12/plaid"
)

var configFile = flag.String("config", "config.toml", "Path to configuration file")

func envToPlaidEnv(env string) plaid.Environment {
	switch env {
	case "development":
		return plaid.Development
	case "sandbox":
		return plaid.Sandbox
	case "production":
		return plaid.Production
	}

	return ""
}

func main() {
	flag.Parse()
	godotenv.Load()

	appConfig := expenses.AppConfig{}
	_, err := toml.DecodeFile(*configFile, &appConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Environment %s\n", appConfig.Environment)
	penv := envToPlaidEnv(appConfig.Environment)
	if penv == "" {
		log.Fatalf("Unrecognized environment %q\n", penv)
	}

	config := plaid.NewConfiguration()
	config.UseEnvironment(penv)
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
