package main

import (
	"flag"
	"log"

	"github.com/BurntSushi/toml"
	expenses "github.com/chriskillpack/expense-tracker"
	"github.com/plaid/plaid-go/v12/plaid"
)

var configFile = flag.String("config", "config.toml", "Path to configuration file")

func main() {
	flag.Parse()

	appConfig := expenses.AppConfig{}
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
