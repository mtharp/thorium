package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

var (
	cmu   sync.Mutex
	cwait = make(map[*http.Request]chan struct{})
	cur   matchRecord
)

func currentMatchNotify(rec matchRecord) {
	cmu.Lock()
	cur = rec
	for _, waitch := range cwait {
		waitch <- struct{}{}
	}
	cmu.Unlock()
}

func (s *TokenServer) viewCurrent(rw http.ResponseWriter, req *http.Request) {
	p1 := req.FormValue("p1")
	p2 := req.FormValue("p2")
	waitch := make(chan struct{}, 1)
	cmu.Lock()
	rec := cur
	if rec.Name1 != p1 || rec.Name2 != p2 {
		cmu.Unlock()
		blob, _ := json.Marshal(rec)
		rw.Write(blob)
		return
	}
	cwait[req] = waitch
	cmu.Unlock()
	defer func() {
		cmu.Lock()
		delete(cwait, req)
		cmu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	select {
	case <-waitch:
	case <-ctx.Done():
	}
	cmu.Lock()
	rec = cur
	cmu.Unlock()
	blob, _ := json.Marshal(rec)
	rw.Write(blob)
}
