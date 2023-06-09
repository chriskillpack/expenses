package expenses

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"

	"github.com/plaid/plaid-go/v12/plaid"
)

type Server struct {
	client *plaid.APIClient
	db     *DB
	s      *http.Server
	mux    *http.ServeMux

	certFile, keyFile string
}

var (
	//go:embed tmpl/*.html
	embeddedFS embed.FS

	indexTmpl *template.Template
)

func init() {
	indexTmpl = template.Must(template.ParseFS(embeddedFS, "tmpl/index.html"))
}

func NewServer(client *plaid.APIClient, db *DB, port int, certFile, keyFile string) *Server {
	srv := &Server{
		client:   client,
		db:       db,
		certFile: certFile,
		keyFile:  keyFile,
	}
	mux := http.NewServeMux()
	mux.Handle("/get_access_token", srv.getAccessToken())
	mux.Handle("/create_link_token", srv.createLinkToken())
	mux.Handle("/admin/institutions/refresh", srv.refreshInstitutions())
	mux.Handle("/admin/transactions/sync", srv.syncTransactions())
	mux.Handle("/static/", http.FileServer(http.Dir("")))
	mux.Handle("/", srv.serveRoot())

	srv.mux = mux
	srv.s = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: srv.mux,
	}

	return srv
}

func (srv *Server) serveRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			items, _ := srv.db.RetrieveItems(req.Context())
			indexTmpl.Execute(w, items)
			return
		}

		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

func (srv *Server) createLinkToken() http.HandlerFunc {
	type response struct {
		ErrorMsg  string `json:",omitempty"`
		LinkToken string `json:",omitempty"`
	}

	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		var resp response

		user := plaid.LinkTokenCreateRequestUser{ClientUserId: "1"}
		ltcreq := plaid.NewLinkTokenCreateRequest(
			"Expense Tracker",
			"en",
			[]plaid.CountryCode{plaid.COUNTRYCODE_US},
			user)
		ltcreq.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
		ltcres, _, err := srv.client.PlaidApi.LinkTokenCreate(req.Context()).LinkTokenCreateRequest(*ltcreq).Execute()
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusBadRequest, resp)
			return
		}

		resp.LinkToken = ltcres.LinkToken
		returnJSON(w, http.StatusCreated, resp)
	}
}

func (srv *Server) getAccessToken() http.HandlerFunc {
	type account struct {
		Id      string
		Name    string
		Mask    string
		Type    string
		Subtype string
	}

	type institution struct {
		Name string
		Id   string `json:"institution_id"`
	}

	type payload struct {
		PublicToken string    `json:"public_token"`
		Accounts    []account `json:"accounts"`
		Institution institution
	}

	type response struct {
		ErrorMsg string `json:",omitempty"`
	}

	return func(w http.ResponseWriter, req *http.Request) {
		var resp response

		pay := payload{}
		err := json.NewDecoder(req.Body).Decode(&pay)
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusInternalServerError, resp)
			return
		}

		// Look up to see if we already have an existing item for this institution
		items, err := srv.db.RetrieveItemsByPlaidInstitutionId(req.Context(), pay.Institution.Id)
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusInternalServerError, resp)
			return
		}
		if len(items) > 0 {
			resp.ErrorMsg = "Institution already linked"
			returnJSON(w, http.StatusBadRequest, resp)
			return
		}

		// No existing item, exchange public token for a private access token with the Plaid API
		ptereq := plaid.NewItemPublicTokenExchangeRequest(pay.PublicToken)
		pteres, _, err := srv.client.PlaidApi.ItemPublicTokenExchange(req.Context()).ItemPublicTokenExchangeRequest(*ptereq).Execute()
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusBadRequest, resp)
			return
		}

		// Exchange successful, store the result in the DB
		err = srv.db.CreateNewItem(req.Context(), pteres.ItemId, pteres.AccessToken, pay.Institution.Id)
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusInternalServerError, resp)
			return
		}

		returnJSON(w, http.StatusOK, resp)
	}
}

func (srv *Server) syncTransactions() http.HandlerFunc {
	type response struct {
		ErrorMsg            string `json:",omitempty"`
		TransactionsAdded   int    `json:"transactions_added"`
		TransactionsRemoved int    `json:"transactions_removed"`
	}

	return func(w http.ResponseWriter, req *http.Request) {
		var resp response

		items, err := srv.db.RetrieveItems(req.Context())
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusInternalServerError, resp)
			return
		}

		for _, item := range items {
			cursor, err := srv.db.GetItemCursorOrNil(req.Context(), item.PlaidItemId)
			if err != nil {
				resp.ErrorMsg = err.Error()
				returnJSON(w, http.StatusInternalServerError, resp)
				return
			}

			hasMore := true
			var added []plaid.Transaction
			var removed []plaid.RemovedTransaction

			for hasMore {
				tsr := plaid.NewTransactionsSyncRequest(item.AccessToken)
				if cursor != "" {
					tsr.SetCursor(cursor)
				}
				tsresp, _, err := srv.client.PlaidApi.TransactionsSync(req.Context()).TransactionsSyncRequest(*tsr).Execute()
				if err != nil {
					resp.ErrorMsg = err.Error()
					returnJSON(w, http.StatusBadRequest, resp)
					return
				}

				hasMore = tsresp.GetHasMore()

				added = append(added, tsresp.GetAdded()...)
				removed = append(removed, tsresp.GetRemoved()...)

				cursor = tsresp.GetNextCursor()
			}

			// Update the cursor
			n, err := srv.db.UpdatePlaidTransactions(req.Context(), added, removed, item.PlaidItemId, cursor)
			if err != nil {
				resp.ErrorMsg = err.Error()
				returnJSON(w, http.StatusInternalServerError, resp)
				return
			}
			resp.TransactionsAdded = int(n)
			resp.TransactionsRemoved = len(removed)
			returnJSON(w, http.StatusOK, resp)
		}
	}
}
func (srv *Server) refreshInstitutions() http.HandlerFunc {
	type response struct {
		ErrorMsg string `json:",omitempty"`
	}

	updateFn := func(id string) error {
		institution, err := srv.db.RetrieveInstitutionById(context.TODO(), id)
		if err != nil {
			log.Printf("Error retrieving institution id %q: %q", id, err)
			return err
		}

		// Skip institutions that already have data
		if institution != nil && institution.Logo != "" && institution.Name != "" {
			return err
		}

		igreq := plaid.NewInstitutionsGetByIdRequest(id, []plaid.CountryCode{plaid.COUNTRYCODE_US})
		igreqo := plaid.NewInstitutionsGetByIdRequestOptions()
		igreqo.SetIncludeOptionalMetadata(true)
		igreq.SetOptions(*igreqo)
		igres, _, err := srv.client.PlaidApi.InstitutionsGetById(context.Background()).InstitutionsGetByIdRequest(*igreq).Execute()
		if err != nil {
			log.Printf("PlaidAPI error %q", err)
			return err
		}

		if institution == nil {
			institution = new(Institution)
		}

		institution.PlaidInstitutionId = igres.Institution.InstitutionId
		institution.Name = igres.Institution.Name
		if igres.Institution.GetLogo() != "" {
			institution.Logo = igres.Institution.GetLogo()
		}

		err = srv.db.UpdateInstitution(context.TODO(), institution)
		if err != nil {
			log.Printf("Failed to update institution %q: %q", id, err)
			return err
		}

		return nil
	}

	return func(w http.ResponseWriter, req *http.Request) {
		var resp response

		// TODO - Is there already another job in progress?

		// Sweep over items to collect all the institution IDs
		iids, err := srv.db.UniqueInstitutionIds(req.Context())
		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusInternalServerError, resp)
			return
		}

		// Update each institution
		for _, iid := range iids {
			select {
			case <-req.Context().Done():
				err = req.Context().Err()
			default:
				err = updateFn(iid)
			}

			if err != nil {
				break
			}
		}

		if err != nil {
			resp.ErrorMsg = err.Error()
			returnJSON(w, http.StatusInternalServerError, resp)
			return
		}

		returnJSON(w, http.StatusOK, resp)
	}
}

func returnJSON(w http.ResponseWriter, statusCode int, data any) error {
	js, err := json.Marshal(data)
	if err != nil {
		// TODO: handle differently? for now replace with a generic error object
		data = struct {
			ErrorMsg string
		}{err.Error()}
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "text/javascript")
	w.Header().Set("Content-Length", strconv.Itoa(len(js)+1))
	w.WriteHeader(statusCode)
	w.Write(js)
	w.Write([]byte("\n"))

	return err
}

func (srv *Server) Start() error {
	err := srv.s.ListenAndServeTLS(srv.certFile, srv.keyFile)
	if err != nil {
		return err
	}

	return nil
}
