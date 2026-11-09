package main

// Intermediar catre Monitorul Preturilor (Consiliul Concurentei): preturile la raft
// din magazinele din jur. Site-ul nu permite apeluri directe din pagina web (CORS),
// asa ca aplicatia locala le face in locul browserului. Doar cateva cereri, doar citire.

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const mpBase = "https://monitorulpreturilor.info/pmonsvc/Retail/"

var mpAllowed = map[string]bool{
	"GetCatalogProductsByNameNetwork": true,
	"GetStoresForProductsByLatLon":    true,
	"GetUATByName":                    true,
	"GetRetailNetworks":               true,
}

type mpEntry struct {
	body []byte
	at   time.Time
}

type mpProxy struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]mpEntry
}

func newMPProxy() *mpProxy {
	return &mpProxy{client: &http.Client{Timeout: 25 * time.Second}, cache: map[string]mpEntry{}}
}

func (m *mpProxy) handler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/mp/")
	if name == "ping" {
		w.Write([]byte("ok"))
		return
	}
	if !mpAllowed[name] {
		http.Error(w, "nepermis", http.StatusForbidden)
		return
	}
	key := name + "?" + r.URL.RawQuery
	m.mu.Lock()
	if e, ok := m.cache[key]; ok && time.Since(e.at) < 30*time.Minute {
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(e.body)
		return
	}
	m.mu.Unlock()
	req, _ := http.NewRequest("GET", mpBase+key, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OferteRomania/1.2 (aplicatie personala)")
	resp, err := m.client.Do(req)
	if err != nil {
		http.Error(w, "Monitorul Preturilor nu raspunde", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Monitorul Preturilor: eroare", http.StatusBadGateway)
		return
	}
	m.mu.Lock()
	if len(m.cache) > 500 {
		m.cache = map[string]mpEntry{}
	}
	m.cache[key] = mpEntry{body, time.Now()}
	m.mu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}
