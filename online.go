package main

// Descarcă la pornire cea mai nouă listă de oferte din GitHub (actualizată zilnic de bot).
// Dacă nu există internet sau repository-ul nu e public, se folosesc ofertele locale.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const onlineDataURL = "https://raw.githubusercontent.com/BM2011/oferte-romania/main/web/data.json"

type onlineData struct {
	mu   sync.Mutex
	data []byte
	path string
}

func newOnlineData(dir string) *onlineData {
	o := &onlineData{path: filepath.Join(dir, "data-online.json")}
	if b, err := os.ReadFile(o.path); err == nil && json.Valid(b) {
		o.data = b // ultima listă descărcată, până vine cea nouă
	}
	go o.refresh()
	return o
}

func (o *onlineData) refresh() {
	cli := &http.Client{Timeout: 15 * time.Second}
	res, err := cli.Get(onlineDataURL + "?t=" + fmt.Sprint(time.Now().Unix()))
	if err != nil {
		fmt.Println("Oferte: nu pot descarca lista noua (fara internet?) - folosesc ofertele locale.")
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fmt.Printf("Oferte: GitHub a raspuns %d - folosesc ofertele locale.\n", res.StatusCode)
		return
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil || !json.Valid(b) {
		fmt.Println("Oferte: lista descarcata e invalida - folosesc ofertele locale.")
		return
	}
	o.mu.Lock()
	o.data = b
	o.mu.Unlock()
	_ = os.WriteFile(o.path, b, 0644)
	var meta struct {
		Updated string `json:"updated"`
		Offers  []any  `json:"offers"`
	}
	_ = json.Unmarshal(b, &meta)
	fmt.Printf("Oferte: lista noua descarcata din GitHub (%d oferte, actualizata %s).\n", len(meta.Offers), meta.Updated)
}

func (o *onlineData) get() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.data
}
