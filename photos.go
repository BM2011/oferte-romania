package main

// Fotografii de produs din Open Food Facts (bază de date deschisă, imagini CC BY-SA).
// Căutările se fac rar (max ~9/minut, conform regulilor OFF), iar rezultatele se păstrează
// în images-cache.json lângă executabil, deci doar prima pornire durează.
// La pornire, serverul caută în fundal fotografiile pentru toate produsele din data.json.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const userAgent = "OferteRomania/1.1 (proiect personal de test)"

type photoCache struct {
	mu   sync.Mutex
	path string
	data map[string]string // q -> url ("" = nu s-a găsit)
	turn sync.Mutex        // serializează cererile spre OFF
	last time.Time
	cli  *http.Client
}

func newPhotoCache(dir string) *photoCache {
	pc := &photoCache{path: filepath.Join(dir, "images-cache.json"), data: map[string]string{},
		cli: &http.Client{Timeout: 30 * time.Second}}
	if b, err := os.ReadFile(pc.path); err == nil {
		_ = json.Unmarshal(b, &pc.data)
	}
	return pc
}

func (pc *photoCache) save() {
	b, _ := json.MarshalIndent(pc.data, "", " ")
	if err := os.WriteFile(pc.path, b, 0644); err != nil {
		fmt.Println("  ! nu pot salva images-cache.json:", err)
	}
}

func simplify(s string) string {
	return strings.NewReplacer("ă", "a", "â", "a", "î", "i", "ș", "s", "ş", "s", "ț", "t", "ţ", "t",
		"ö", "o", "ü", "u", "ä", "a", "é", "e", "è", "e").Replace(strings.ToLower(s))
}

type offProduct struct {
	Name  string `json:"product_name"`
	Brand any    `json:"brands"` // text sau listă, în funcție de API
	Small string `json:"image_front_small_url"`
	Big   string `json:"image_front_url"`
}

func (p offProduct) brandText() string {
	switch b := p.Brand.(type) {
	case string:
		return b
	case []any:
		var s []string
		for _, x := range b {
			s = append(s, fmt.Sprint(x))
		}
		return strings.Join(s, " ")
	}
	return ""
}

func (pc *photoCache) get(api string) ([]offProduct, error) {
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", userAgent)
	res, err := pc.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var out struct {
		Products []offProduct `json:"products"`
		Hits     []offProduct `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return append(out.Products, out.Hits...), nil
}

func pick(ps []offProduct, brand string) string {
	for _, p := range ps {
		img := p.Big
		if img == "" {
			img = p.Small
		}
		if img != "" && strings.Contains(simplify(p.brandText()+" "+p.Name), brand) {
			return img
		}
	}
	return ""
}

func (pc *photoCache) cached(q string) (string, bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	u, ok := pc.data[q]
	return u, ok
}

func (pc *photoCache) lookup(q string) string {
	if u, ok := pc.cached(q); ok {
		return u
	}
	pc.turn.Lock()
	defer pc.turn.Unlock()
	if u, ok := pc.cached(q); ok { // rezolvat între timp
		return u
	}
	if wait := 6500*time.Millisecond - time.Since(pc.last); wait > 0 {
		time.Sleep(wait)
	}
	pc.last = time.Now()

	brand := strings.Fields(simplify(q))[0]
	enc := url.QueryEscape(q)
	fields := "product_name,brands,image_front_small_url,image_front_url"
	// 1) motorul nou de căutare OFF; 2) căutarea clasică, ca rezervă
	sources := []string{
		"https://search.openfoodfacts.org/search?page_size=10&fields=" + fields + "&q=" + enc,
		"https://world.openfoodfacts.org/cgi/search.pl?search_simple=1&json=1&page_size=10&fields=" + fields + "&search_terms=" + enc,
	}
	found, anyOK := "", false
	var lastErr error
	for _, api := range sources {
		ps, err := pc.get(api)
		if err != nil {
			lastErr = err
			continue
		}
		anyOK = true
		if found = pick(ps, brand); found != "" {
			break
		}
	}
	if !anyOK {
		fmt.Printf("  ! foto %-28s eroare de retea (%v) - reincerc la urmatoarea pornire\n", q, lastErr)
		return ""
	}
	if found != "" {
		fmt.Printf("  + foto %-28s gasita\n", q)
	} else {
		fmt.Printf("  - foto %-28s nu exista in Open Food Facts (raman iconita)\n", q)
	}
	pc.mu.Lock()
	pc.data[q] = found
	pc.save()
	pc.mu.Unlock()
	return found
}

// prefetch caută în fundal fotografiile pentru toate produsele din data.json.
func (pc *photoCache) prefetch(root fs.FS) {
	b, err := fs.ReadFile(root, "data.json")
	if err != nil {
		return
	}
	var d struct {
		Offers []struct {
			Q   string `json:"q"`
			Img string `json:"img"`
		} `json:"offers"`
	}
	if json.Unmarshal(b, &d) != nil {
		return
	}
	seen := map[string]bool{}
	var todo []string
	for _, o := range d.Offers {
		q := strings.TrimSpace(o.Q)
		if q == "" || o.Img != "" || seen[q] {
			continue
		}
		seen[q] = true
		if _, ok := pc.cached(q); !ok {
			todo = append(todo, q)
		}
	}
	if len(todo) == 0 {
		fmt.Println("Fotografii: toate sunt deja salvate.")
		return
	}
	fmt.Printf("Fotografii: caut %d produse in Open Food Facts (~%d min). Pagina se actualizeaza singura.\n",
		len(todo), (len(todo)*13/2)/60+1)
	for _, q := range todo {
		pc.lookup(q)
	}
	fmt.Println("Fotografii: cautarea s-a terminat.")
}

func (pc *photoCache) handler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if q == "" || len(q) > 80 {
		w.Write([]byte(`{"url":""}`))
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"url": pc.lookup(q)})
}
