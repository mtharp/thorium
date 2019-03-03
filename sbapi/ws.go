package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

const (
	wsURL    = "ws://www-cdn-twitch.saltybet.com:1337/socket.io/?EIO=3&transport=websocket"
	stateURL = "http://www.saltybet.com/state.json"
	dataURL  = "http://www.saltybet.com/zdata.json"

	minRetry      = 1
	maxRetry      = 60
	backoffFactor = 3

	pingInterval = 15 * time.Second
	pingTimeout  = 3*pingInterval + 1*time.Second
	staleTimeout = 15 * time.Minute

	fetchHoldoff = time.Second
)

var (
	lastModified = make(map[string]string)
	watching     = make(map[string]bool)

	modeMatch = map[string]string{
		"until the next tournament":          "matchmaking",
		"Tournament mode will be activated":  "matchmaking",
		"characters are left in the bracket": "tournament",
		"FINAL ROUND":                        "tournament",
		"exhibition matches left":            "exhibitions",
		"Matchmaking mode will be activated": "exhibitions",
	}
)

func subWS(ch chan struct{}) {
	delayRetry := minRetry
	pongch := make(chan bool)
	first := true
	for {
		if !first {
			time.Sleep(time.Duration(delayRetry) * time.Second)
			delayRetry *= backoffFactor
			if delayRetry > maxRetry {
				delayRetry = maxRetry
			}
		}
		first = false
		log.Printf("connecting to %s", wsURL)
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Println("error:", err)
			continue
		}
		donech := make(chan struct{})
		// start sending keepalive pings
		go keepalive(c, pongch, donech)
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				break
			}
			var gotData bool
			if bytes.HasPrefix(msg, []byte("42")) {
				select {
				case ch <- struct{}{}:
				default:
				}
				gotData = true
			}
			select {
			case pongch <- gotData:
			default:
			}
		}
		close(donech)
		c.Close()
	}
}

func keepalive(conn *websocket.Conn, pongch chan bool, donech chan struct{}) {
	lastPong := time.NewTimer(pingTimeout)
	defer lastPong.Stop()
	stale := time.NewTimer(staleTimeout)
	defer stale.Stop()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-donech:
			return
		case <-ping.C:
			conn.WriteMessage(1, []byte("2"))
		case gotData := <-pongch:
			lastPong.Reset(pingTimeout)
			if gotData {
				stale.Reset(staleTimeout)
			}
		case <-lastPong.C:
			log.Printf("error: websocket timed out")
			conn.Close()
			return
		case <-stale.C:
			log.Printf("error: no data from websocket for %s, reconnecting")
			conn.Close()
			return
		}
	}
}

var cli *http.Client

func main() {
	viper.AutomaticEnv()
	for _, name := range viper.GetStringSlice("watch") {
		watching[name] = true
	}
	if err := connectDB(); err != nil {
		log.Fatalln("error: can't connect to db:", err)
	}
	// watch websocket
	ch := make(chan struct{}, 1)
	go subWS(ch)
	jar, _ := cookiejar.New(nil)
	cli = &http.Client{Jar: jar}
	rt := time.NewTimer(time.Second)
	for {
		select {
		case <-ch:
			rt.Reset(fetchHoldoff)
		case <-rt.C:
			if err := update(db); err != nil {
				log.Printf("error updating state: %s", err)
			}
		}
	}
}

func fetch(url string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	previous := lastModified[req.URL.Path]
	for i := 0; i < 5; i++ {
		if i > 0 {
			time.Sleep(1 * time.Second)
		}
		if i == 4 {
			previous = ""
		}
		d, err := fetchOnce(req, previous)
		if err != nil {
			return nil, err
		} else if d != nil {
			return d, nil
		}
	}
	return nil, errors.New("not updated")
}

func fetchOnce(req *http.Request, previous string) (map[string]interface{}, error) {
	req.Header.Set("If-Modified-Since", previous)
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	blob, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		log.Printf("%s not updated yet, trying again", req.URL)
		return nil, nil
	case http.StatusOK:
		lm := resp.Header.Get("Last-Modified")
		if lm == previous {
			log.Printf("%s not updated yet, trying again", req.URL)
			return nil, nil
		}
		lastModified[req.URL.Path] = lm
		var d map[string]interface{}
		if err := json.Unmarshal(blob, &d); err != nil {
			return nil, fmt.Errorf("unmarshalling %s: %s", path.Base(req.URL.Path), err)
		}
		return d, nil
	default:
		return nil, fmt.Errorf("HTTP %s %s:\n%s", resp.Status, resp.Request.URL, blob)
	}
}

type playerData struct {
	bank, wager, win int64
	player           string
}

var (
	lastStatus     string
	lastP1, lastP2 string
	mode           string
	banks          = make(map[string]playerData)
)

func strfield(d interface{}) string {
	s, _ := d.(string)
	return s
}

func intfield(d interface{}) int64 {
	n, _ := strconv.ParseInt(strings.Replace(strfield(d), ",", "", -1), 10, 64)
	return n
}

func update(db *DB) error {
	d, err := fetch(stateURL)
	if err != nil {
		return err
	}
	status := strfield(d["status"])
	if status == lastStatus {
		return nil
	}
	lastStatus = status
	switch status {
	case "locked":
		d, err = fetch(dataURL)
		if err != nil {
			return err
		}
		lastP1 = strfield(d["p1name"])
		lastP2 = strfield(d["p2name"])
		p1total := intfield(d["p1total"])
		p2total := intfield(d["p2total"])
		mode = ""
		rem := strfield(d["remaining"])
		for mstr, mmode := range modeMatch {
			if strings.Contains(rem, mstr) {
				mode = mmode
			}
		}
		for k := range banks {
			delete(banks, k)
		}
		for _, iattrs := range d {
			attrs, _ := iattrs.(map[string]interface{})
			name := strfield(attrs["n"])
			b := playerData{
				bank:   intfield(attrs["b"]),
				wager:  intfield(attrs["w"]),
				player: strfield(attrs["p"]),
			}
			n1, n2 := lastP1, lastP2
			if b.player == "2" {
				b.win = (b.wager*p1total + p2total - 1) / p2total
				n2 = "<" + n2 + ">"
			} else {
				b.win = (b.wager*p2total + p1total - 1) / p1total
				n1 = "<" + n1 + ">"
			}
			banks[name] = b
			if watching[name] {
				log.Printf("[%11s] %s %d bets %d : %s : %s", mode, name, b.bank, b.wager, n1, n2)
			}
		}
	case "1", "2":
		if lastP1 == "" {
			return nil
		} else if strfield(d["p1name"]) != lastP1 || strfield(d["p2name"]) != lastP2 {
			return errors.New("player mismatch")
		}
		for name, data := range banks {
			change := -data.wager
			result := "lose"
			if data.player == status {
				change = data.win
				result = "wins"
			}
			data.bank += change
			if mode != "tournament" {
				db.SetBank(name, data.bank)
			}
			if watching[name] {
				log.Printf("[%11s] %s %s %+d -> %d", mode, name, result, change, data.bank)
				if mode != "tournament" {
					db.AddHistory(name, data.bank)
				}
			}
		}
	}
	return nil
}
