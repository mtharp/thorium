package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"

	goirc "github.com/fluffle/goirc/client"
	"golang.org/x/oauth2"
)

const (
	ircHost    = "irc.chat.twitch.tv"
	ircPort    = "6697"
	ircChannel = "#saltybet"
	bot1Name   = "waifu4u"
	bot2Name   = "saltybet"
)

var (
	lineOpen   = regexp.MustCompile(`Bets are OPEN for (.*) vs (.*)! \((?:(.*) Tier|Requested by .*?)\)(?: \(.*\))? (?:\((.*)\) www.saltybet.com|tournament bracket.*)$`)
	closedPart = `.*?(?:\(([^)]+)\) )?- \$(.*)`
	tierPart   = `(?:.|None)`
	lineClosed = regexp.MustCompile(`Bets are locked\. ` + closedPart + `, ` + closedPart)
	linePaid   = regexp.MustCompile(`.* wins! Payouts to Team (.*)\. (.*)!`)
	lineMode   = regexp.MustCompile(`^(Tournament|Matchmaking|Exhibitions) will start shortly`)
	lineIgnore = regexp.MustCompile(`^(wtfSalt |wtfVeku Note:|Current pot|Current stage|Current odds|Download WAIFU Wars|.* by.*, .* by.*|` + tierPart + `(?: / ` + tierPart + `)? Tier$|The current game mode is:|The current tournament bracket|Palettes of previous match:|.* vs .* was requested by)`)
)

func runIRC(ts oauth2.TokenSource) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t, err := ts.Token()
	if err != nil {
		return fmt.Errorf("can't connect to IRC: %s", err)
	}
	ic := goirc.NewConfig("thorium", "thorium", "thorium saltbot") // nick is ignored
	ic.Server = net.JoinHostPort(ircHost, ircPort)
	ic.SSL = true
	ic.SSLConfig = &tls.Config{ServerName: ircHost}
	ic.Pass = "oauth:" + t.AccessToken

	cl := goirc.Client(ic)
	cl.HandleFunc(goirc.CONNECTED, func(conn *goirc.Conn, line *goirc.Line) {
		log.Println("connected")
		conn.Join(ircChannel)
	})
	cl.HandleFunc(goirc.DISCONNECTED, func(conn *goirc.Conn, line *goirc.Line) {
		cancel()
	})
	cl.HandleFunc(goirc.PRIVMSG, func(conn *goirc.Conn, line *goirc.Line) {
		if line.Nick != bot1Name && line.Nick != bot2Name {
			return
		}
		text := line.Text()
		if m := lineOpen.FindStringSubmatch(text); m != nil {
			log.Printf("bets open: red=%s blue=%s tier=%s mode=%s", m[1], m[2], m[3], m[4])
		} else if m := lineClosed.FindStringSubmatch(text); m != nil {
			log.Printf("bets locked: streakRed=%s potRed=%s streakBlue=%s potBlue=%s", m[1], m[2], m[3], m[4])
		} else if m := linePaid.FindStringSubmatch(text); m != nil {
			log.Printf("match over: winner=%s remaining=%s", m[1], m[2])
		} else if m := lineMode.FindStringSubmatch(text); m != nil {
			log.Printf("match over: mode=%s", strings.ToLower(m[1]))
		} else if !lineIgnore.MatchString(text) {
			log.Printf("%q", text)
		}
	})

	log.Println("attempting connection to", ic.Server)
	if err := cl.Connect(); err != nil {
		return fmt.Errorf("can't connect to IRC: %s", err)
	}
	<-ctx.Done()
	log.Printf("warning: IRC disconnected")
	return nil
}
