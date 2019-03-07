package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/twitch"
)

const (
	listenPort    = 9900
	minRetry      = 1 * time.Second
	maxRetry      = 60 * time.Second
	backoffFactor = 3
)

func main() {
	// configure
	viper.AutomaticEnv()
	if err := connectDB(); err != nil {
		log.Fatalln("error: connect to db:", err)
	}
	conf := &oauth2.Config{
		ClientID:     viper.GetString("client_id"),
		ClientSecret: viper.GetString("client_secret"),
		RedirectURL:  viper.GetString("redirect_url"),
		Endpoint:     twitch.Endpoint,
		Scopes:       []string{"chat:read"},
	}
	newToken := make(chan struct{})
	s := &TokenServer{
		Oauth:    conf,
		NewToken: newToken,
	}
	http.HandleFunc("/healthz", s.viewHealth)
	http.HandleFunc("/current", s.viewCurrent)
	http.HandleFunc("/", s.viewCallback)
	var good bool
	t, err := getToken()
	if err != nil {
		log.Fatalln("error: failed to retrieve token:", err)
	} else if t != nil {
		s.source = oauth2.ReuseTokenSource(nil, s.Oauth.TokenSource(context.Background(), t))
		if _, err := s.Token(); err == nil {
			good = true
		}
	}
	go http.ListenAndServe(viper.GetString("listen"), nil)
	if good {
		log.Println("loaded token from db")
	} else {
		log.Println("saved token not available, please log in at", conf.RedirectURL)
		<-newToken
	}
	go s.keepAlive()

	delay := minRetry
	timer := time.NewTimer(delay)
	for {
		select {
		case <-newToken:
		case <-timer.C:
		}
		start := time.Now()
		if err := runIRC(s); err != nil {
			log.Printf("error: %s", err)
		}
		if time.Since(start) > delay*3 {
			delay = minRetry
		} else {
			delay *= backoffFactor
			if delay > maxRetry {
				delay = maxRetry
			}
		}
		timer.Reset(delay)
	}
}

type TokenServer struct {
	Oauth    *oauth2.Config
	Home     string
	NewToken chan<- struct{}

	source oauth2.TokenSource
	state  string
	mu     sync.Mutex
}

func (s *TokenServer) viewCallback(rw http.ResponseWriter, req *http.Request) {
	state := req.FormValue("state")
	code := req.FormValue("code")
	if state == "" || code == "" {
		d := make([]byte, 8)
		rand.Reader.Read(d)
		state := hex.EncodeToString(d)
		authURL := s.Oauth.AuthCodeURL(state)
		s.mu.Lock()
		s.state = state
		s.mu.Unlock()
		fmt.Fprintf(rw, `<!DOCTYPE html>
<html><body><script type="text/javascript">
location.href = "%s";
</script><a href="%s">login to twitch</a></body></html>
`, authURL, authURL)
		return
	}
	if state != req.FormValue("state") {
		http.Error(rw, "wrong oauth state", 400)
		return
	}
	t, err := s.Oauth.Exchange(req.Context(), code)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		return
	}
	s.mu.Lock()
	s.state = ""
	s.source = oauth2.ReuseTokenSource(nil, s.Oauth.TokenSource(context.Background(), t))
	s.mu.Unlock()
	s.Token() // trigger token save
	select {
	case s.NewToken <- struct{}{}:
	default:
	}
	rw.Header().Set("Content-Type", "text/plain; charset=utf8")
	fmt.Fprintf(rw, "login complete")
}

func (s *TokenServer) viewHealth(rw http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	conn, err := db.Acquire()
	if err == nil {
		defer db.Release(conn)
		err = conn.Ping(ctx)
		if err == nil {
			fmt.Fprintln(rw, "OK")
			return
		}
	}
	log.Printf("error: in health check:", err)
	http.Error(rw, err.Error(), 500)
}

func (s *TokenServer) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	ts := s.source
	s.mu.Unlock()
	if ts == nil {
		return nil, errors.New("token not available")
	}
	t, err := ts.Token()
	if err != nil {
		return nil, err
	}
	if err := putToken(t); err != nil {
		log.Printf("error: failed to persist token: %s", err)
	}
	return t, nil
}

func (s *TokenServer) keepAlive() {
	t := time.NewTicker(time.Hour)
	for {
		<-t.C
		if _, err := s.Token(); err != nil {
			log.Printf("error: token refresh failed: %s", err)
			log.Printf("Login page:", s.Home)
		}
	}
}
