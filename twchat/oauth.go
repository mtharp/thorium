package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/twitch"
)

const (
	listenPort = 9900
)

func main() {
	// configure
	viper.SetConfigName("thorium")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("error:", err)
	}
	home := fmt.Sprintf("http://localhost:%d", listenPort)
	conf := &oauth2.Config{
		ClientID:     viper.GetString("twitch.client_id"),
		ClientSecret: viper.GetString("twitch.client_secret"),
		Endpoint:     twitch.Endpoint,
		RedirectURL:  home + "/cb",
		Scopes:       []string{"chat:read"},
	}
	newToken := make(chan struct{})
	s := &TokenServer{
		Oauth:     conf,
		Home:      home,
		TokenFile: viper.GetString("twitch.token_file"),
		NewToken:  newToken,
	}
	http.HandleFunc("/", s.viewHome)
	http.HandleFunc("/cb", s.viewCallback)
	go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", listenPort), nil)
	go s.keepAlive()

	timer := time.NewTimer(time.Minute)
	for {
		select {
		case <-newToken:
		case <-timer.C:
		}
		if err := runIRC(s); err != nil {
			log.Printf("error: %s", err)
		}
	}
}

type TokenServer struct {
	Oauth     *oauth2.Config
	Home      string
	TokenFile string
	NewToken  chan<- struct{}

	source oauth2.TokenSource
	state  string
	mu     sync.Mutex
}

func (s *TokenServer) viewHome(rw http.ResponseWriter, req *http.Request) {
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
}

func (s *TokenServer) viewCallback(rw http.ResponseWriter, req *http.Request) {
	if s.state != req.FormValue("state") {
		http.Error(rw, "wrong oauth state", 400)
		return
	}
	t, err := s.Oauth.Exchange(req.Context(), req.FormValue("code"))
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
	if s.TokenFile != "" {
		blob, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(s.TokenFile, blob, 0600); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (s *TokenServer) keepAlive() {
	var good bool
	if s.TokenFile != "" {
		blob, err := ioutil.ReadFile(s.TokenFile)
		if err == nil {
			t := new(oauth2.Token)
			if err := json.Unmarshal(blob, t); err == nil {
				s.mu.Lock()
				s.source = oauth2.ReuseTokenSource(nil, s.Oauth.TokenSource(context.Background(), t))
				s.mu.Unlock()
			}
			if _, err := s.Token(); err == nil {
				good = true
				select {
				case s.NewToken <- struct{}{}:
				default:
				}
			}
		}
	}
	if good {
		log.Println("loaded token from file")
	} else {
		log.Println("saved token not available, please log in:\n", s.Home)
	}
	t := time.NewTicker(time.Hour)
	for {
		<-t.C
		if _, err := s.Token(); err != nil {
			log.Printf("error: token refresh failed: %s", err)
			log.Printf("Login page:", s.Home)
		}
	}
}
