package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsURL   = "ws://www-cdn-twitch.saltybet.com:1337/socket.io/?EIO=3&transport=websocket"
	dataURL = "http://www.saltybet.com/zdata.json"

	minRetry      = 1
	maxRetry      = 60
	backoffFactor = 3

	fetchHoldoff = 5 * time.Second
)

func subWS(ch chan struct{}) {
	delayRetry := minRetry
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
		go keepalive(c, donech)
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				break
			}
			if bytes.HasPrefix(msg, []byte("42")) {
				log.Printf(".")
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
		close(donech)
		c.Close()
	}
}

func keepalive(conn *websocket.Conn, donech chan struct{}) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-donech:
			return
		case <-t.C:
			conn.WriteMessage(1, []byte("2"))
		}
	}
}

type state struct {
	Status string `json:"status"`
}

func main() {
	ch := make(chan struct{}, 1)
	go subWS(ch)
	jar, _ := cookiejar.New(nil)
	cli := &http.Client{Jar: jar}
	rt := time.NewTimer(time.Second)
	for {
		select {
		case <-ch:
			rt.Reset(fetchHoldoff)
		case <-rt.C:
			log.Printf("getting %s", dataURL)
			resp, err := cli.Get(dataURL)
			if err != nil {
				log.Printf("error: %s", err)
				continue
			}
			data, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 {
				log.Printf("error: HTTP %s:\n%s", resp.Status, data)
				continue
			}
			var st state
			if err := json.Unmarshal(data, &st); err != nil {
				log.Printf("error: unmarshalling zdata.json: %s", err)
				continue
			}
			log.Printf("status: %s", st.Status)
		}
	}
}
