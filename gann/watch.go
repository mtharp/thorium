package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	deep "github.com/patrikeh/go-deep"
	"github.com/spf13/viper"
)

func watchAndRun(nn *deep.Neural) {
	var p1name, p2name string
	var sbs struct{ Bank int64 }
	var mst struct{ P1, P2, Tier string }
	var failures int
	jar, _ := cookiejar.New(nil)
	cli := &http.Client{Jar: jar}
	for {
		blob, _ := ioutil.ReadFile("/tmp/mstate.json")
		json.Unmarshal(blob, &mst)
		if mst.P1 == p1name && mst.P2 == p2name {
			time.Sleep(time.Second)
			continue
		}
		p1name = mst.P1
		p2name = mst.P2
		blob, _ = ioutil.ReadFile("/tmp/sbstate.json")
		json.Unmarshal(blob, &sbs)
		idx, ok := tierIdx[mst.Tier]
		if !ok {
			continue
		}
		d := tiers[idx]

		if _, ok := d.chars[p1name]; !ok {
			log.Printf("no data for %q", p1name)
			continue
		}
		if _, ok := d.chars[p2name]; !ok {
			log.Printf("no data for %q", p2name)
			continue
		}
		rec := newLiveRecord(mst.Tier, p1name, p2name)
		o := nn.Predict(d.BetVector(rec))
		j, k := o[0], o[1]
		wk := j
		if k > j {
			wk = k
		}
		if wk < 0 {
			wk = 0
		}
		bank := float64(sbs.Bank)
		wager := bank * baseBet * wk
		bailout := 100.0
		if bank-wager < bailout || wager > bank {
			wager = bank
		}
		idx = 0
		if k > j {
			idx = 1
		}
		iwager := int(wager / 5)
		log.Printf("Placing %dk on %q", iwager/1000, rec.Name[idx])
		var p int
		if rec.Name[idx] == p1name {
			p = 1
		} else if rec.Name[idx] == p2name {
			p = 2
		} else {
			continue
		}
		time.Sleep(5 * time.Second)
		if err := postWager(cli, p, iwager); err != nil {
			log.Printf("error: %s", err)
			failures++
		} else {
			failures = 0
		}
		if failures > 10 {
			break
		}
	}
}

func postWager(cli *http.Client, player, wager int) error {
	v := make(url.Values)
	v.Set("selectedplayer", fmt.Sprintf("player%d", player))
	v.Set("wager", strconv.Itoa(wager))
	body := strings.NewReader(v.Encode())
	req, err := http.NewRequest("POST", "http://www.saltybet.com/ajax_place_bet.php", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Referer", "http://www.saltybet.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36")
	if v := viper.GetString("sessid"); v != "" {
		req.Header.Set("Cookie", "PHPSESSID="+v)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	respb, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode == 200 {
		if len(respb) == 1 && respb[0] == '1' {
			log.Printf("success")
			return nil
		}
		return fmt.Errorf("unexpected response %x", respb)
	}
	return fmt.Errorf("HTTP %s %s:\n%s", resp.Status, resp.Request.URL, respb)
}
