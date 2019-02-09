package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	deep "github.com/patrikeh/go-deep"
	"github.com/spf13/viper"
)

func watchAndRun(nns []*deep.Neural, ts time.Time) {
	var p1name, p2name string
	var mst struct{ P1, P2, Tier, Mode string }
	var failures int
	jar, _ := cookiejar.New(nil)
	cli := &http.Client{Jar: jar}
	uid, bank, err := scrapeHome(cli)
	if err != nil {
		log.Fatalln("error:", err)
	}
	log.Printf("scraped uid=%s bank=%f", uid, bank)
	var bankChanged, modeChanged bool
	lastMode := ""
	for {
		// wait for tier info
		blob, _ := ioutil.ReadFile("/tmp/mstate.json")
		json.Unmarshal(blob, &mst)
		if mst.P1 == p1name && mst.P2 == p2name {
			time.Sleep(time.Second)
			continue
		}
		p1name = mst.P1
		p2name = mst.P2
		modeChanged = lastMode != "" && lastMode != mst.Mode
		if modeChanged {
			bankChanged = true
		}
		lastMode = mst.Mode
		// update bank
		if bankChanged {
			zbank, err := getBank(cli, uid)
			if err != nil {
				log.Printf("error: %s", err)
				failures++
				continue
			}
			if zbank <= 0 {
				_, sbank, err := scrapeHome(cli)
				if err != nil {
					log.Printf("error: %s", err)
					failures++
					continue
				}
				bank = sbank
				log.Printf("bank from scrape: %f", sbank)
			} else {
				bank = zbank
				log.Printf("bank from zdata: %f", zbank)
			}
			bankChanged = false
		}
		// update character data
		tierRecs, ts2, err := getRecords("matches", ts)
		if err != nil {
			log.Printf("error: fetching new match records: %s", err)
		} else {
			for tier, recs := range tierRecs {
				tiers[tierIdx[tier]].chars.Update(recs)
				log.Printf("Added %d match record(s) to tier %s", len(recs), tier)
			}
			if ts2.After(ts) {
				ts = ts2
			}
		}

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
		v := d.BetVector(rec, bank)
		wl := make(wagerList, len(nns))
		for i, nn := range nns {
			wl[i] = wagerFromVector(nn.Predict(v))
		}
		wg := wl.Consensus()
		if wg.Size() < 0.002 {
			log.Printf("too close to call")
			continue
		}

		wager := bank * baseBet * wg.Size()
		log.Printf("base bet %f", wager)
		bailout := float64(defaultBailout)
		switch mst.Mode {
		case "matchmaking":
			wager *= mmScale
		case "exhibitions":
			wager *= exhibScale
		case "tournament":
			bailout = tournBailout
		default:
			log.Printf("unknown mode %q", mst.Mode)
			continue
		}
		if bank-wager < bailout || wager > bank || bank < alwaysAllIn {
			wager = bank
		}
		if wager > maxBet {
			wager = maxBet
		}
		log.Printf("adjusted %f", wager)
		idx = 0
		if wg.PredictB() {
			idx = 1
		}
		iwager := int(wager)
		dwager := iwager
		suffix := ""
		if iwager >= 10000 {
			dwager /= 1000
			suffix = "k"
		}
		log.Printf("Placing %d%s on %q", dwager, suffix, rec.Name[idx])
		//continue
		var p int
		if rec.Name[idx] == p1name {
			p = 1
		} else if rec.Name[idx] == p2name {
			p = 2
		} else {
			continue
		}
		time.Sleep(5 * time.Second)
		bankChanged = true
		if err := postWager(cli, p, iwager); err != nil {
			log.Printf("error placing bet: %s", err)
			failures++
		} else {
			failures = 0
		}
		if failures > 10 {
			break
		}
	}
}

func strfield(d interface{}) string {
	s, _ := d.(string)
	return s
}

func intfield(d interface{}) int64 {
	n, _ := strconv.ParseInt(strings.Replace(strfield(d), ",", "", -1), 10, 64)
	return n
}

func getBank(cli *http.Client, uid string) (float64, error) {
	req, _ := http.NewRequest("GET", "http://www.saltybet.com/zdata.json", nil)
	blob, err := do(cli, req)
	if err != nil {
		return 0, err
	}
	var d map[string]interface{}
	if err := json.Unmarshal(blob, &d); err != nil {
		return 0, err
	}
	attrs, _ := d[uid].(map[string]interface{})
	bank := float64(intfield(attrs["b"]))
	return bank, nil
}

func do(cli *http.Client, req *http.Request) ([]byte, error) {
	req.Header.Set("Referer", "http://www.saltybet.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36")
	req.Header.Set("Cookie", "PHPSESSID="+viper.GetString("sessid"))
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	respb, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %s %s:\n%s", resp.Status, resp.Request.URL, respb)
	}
	return respb, nil
}

func postWager(cli *http.Client, player, wager int) error {
	v := make(url.Values)
	v.Set("selectedplayer", fmt.Sprintf("player%d", player))
	v.Set("wager", strconv.Itoa(wager))
	body := strings.NewReader(v.Encode())
	req, _ := http.NewRequest("POST", "http://www.saltybet.com/ajax_place_bet.php", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	blob, err := do(cli, req)
	if err != nil {
		return err
	} else if len(blob) == 0 {
		return errors.New("empty response")
	}
	log.Printf("success")
	return nil
}

var (
	scrapeUid  = regexp.MustCompile(`<input type="hidden".* name="u" value ?="([^"]*)"`)
	scrapeBank = regexp.MustCompile(`<input type="hidden".* name="b" value ?="([^"]*)"`)
)

func scrapeHome(cli *http.Client) (uid string, bank float64, err error) {
	req, _ := http.NewRequest("GET", "http://www.saltybet.com/", nil)
	blob, err := do(cli, req)
	if err != nil {
		return
	}
	m := scrapeUid.FindSubmatch(blob)
	if len(m) < 1 {
		err = errors.New("unable to find uid")
		return
	}
	uid = string(m[1])
	m = scrapeBank.FindSubmatch(blob)
	if len(m) < 1 {
		err = errors.New("unable to find bank")
		return
	}
	bankInt, _ := strconv.ParseInt(string(m[1]), 10, 0)
	bank = float64(bankInt)
	return
}
