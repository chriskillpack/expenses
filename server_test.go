package expenses

import (
	"net/http"

	"github.com/plaid/plaid-go/v12/plaid"
)

// A pass through RoundTrip that allows mocking of Plaid API HTTP calls
type passthruRT int

func (rt passthruRT) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req)
}

func mockPlaidClient() *plaid.APIClient {
	var prt passthruRT
	config := plaid.NewConfiguration()
	config.UseEnvironment(plaid.Development)
	config.AddDefaultHeader("PLAID-CLIENT-ID", "TESTING-ID")
	config.AddDefaultHeader("PLAID-SECRET", "TESTING-SECRET")
	config.HTTPClient = &http.Client{Transport: prt}

	return nil
}
