package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

func handleAuthorize(config *oauth2.Config, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, config.AuthCodeURL(token, oauth2.AccessTypeOnline), http.StatusFound)
	}
}

func handleOAuth2Callback(store *sessions.CookieStore, config *oauth2.Config, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st := r.FormValue("state"); st != token {
			http.Error(w, "Returned state token does not match.", 401)
		}

		t, err := config.Exchange(oauth2.NoContext, r.FormValue("code"))
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		session, _ := store.Get(r, "sndcld")

		session.Values["token"] = t.AccessToken
		session.Save(r, w)

		f, _ := os.Open("./layout.html")
		io.Copy(w, f)
		f.Close()
	}
}

func index(id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open("./layout.html")
		if err != nil {
			http.Error(w, "Failed to load layout.html.", 500)
		}
		io.Copy(w, f)
		f.Close()
	}
}

func getStream(store *sessions.CookieStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "sndcld")

		t, ok := session.Values["token"]
		if !ok {
			http.Error(w, "No token in session", 500)
		}

		resp, err := http.Get(fmt.Sprintf("https://api.soundcloud.com/me/activities?limit=25&oauth_token=%s", t.(string)))
		if err != nil {
			http.Error(w, "No token in session", 500)
		}
		defer resp.Body.Close()

		io.Copy(w, resp.Body)
	}
}

func main() {
	var (
		clientID      = flag.String("client-id", "", "soundcloud client id")
		clientSecret  = flag.String("client-secret", "", "soundcloud client secret")
		port          = flag.String("port", "3000", "address to bind the server on")
		callbackToken = flag.String("callback-token", "testToken", "OAuth token used to protect against CSRF attacks")
		appURL        = flag.String("app-url", "http://localhost:3000", "url of the app")
		store         = sessions.NewCookieStore([]byte("secret key"))
	)
	flag.Parse()

	config := &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		RedirectURL:  *appURL + "/oauth2callback",
		Scopes: []string{
			"non-expiring",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://soundcloud.com/connect",
			TokenURL: "https://api.soundcloud.com/oauth2/token",
		},
	}
	m := pat.New()

	m.Get("/public/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
	})

	m.Get("/stream", getStream(store))
	m.Get("/oauth2callback", handleOAuth2Callback(store, config, *callbackToken))
	m.Post("/authorize", handleAuthorize(config, *callbackToken))
	m.Get("/", index(*clientID))

	handler := handlers.CompressHandler(handlers.LoggingHandler(os.Stdout, m))
	log.Printf("Listening on %s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, handler))
}
