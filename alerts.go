package main

// Alerte pe PC: produsele urmărite și super ofertele noi apar ca notificări Windows.
// Pagina trimite lista de produse urmărite la /api/alerts; programul verifică ofertele
// (descărcate din GitHub) la pornire și apoi la fiecare 3 ore.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type alertSettings struct {
	Watch  []string `json:"watch"`
	Notify bool     `json:"notify"` // notificări Windows activate
	Super  bool     `json:"super"`  // include super ofertele noi
}

type alerts struct {
	mu       sync.Mutex
	dir      string
	set      alertSettings
	seen     map[string]bool
	online   *onlineData
	lastSent []string
}

func newAlerts(dir string, online *onlineData) *alerts {
	a := &alerts{dir: dir, online: online, seen: map[string]bool{}, set: alertSettings{Super: true}}
	if b, err := os.ReadFile(filepath.Join(dir, "alerte.json")); err == nil {
		_ = json.Unmarshal(b, &a.set)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "alerte-vazute.json")); err == nil {
		_ = json.Unmarshal(b, &a.seen)
	}
	go func() {
		time.Sleep(20 * time.Second) // după descărcarea listei noi
		for {
			a.check()
			time.Sleep(3 * time.Hour)
			a.online.refresh()
		}
	}()
	return a
}

func (a *alerts) save() {
	b, _ := json.MarshalIndent(a.set, "", " ")
	_ = os.WriteFile(filepath.Join(a.dir, "alerte.json"), b, 0644)
	b, _ = json.Marshal(a.seen)
	_ = os.WriteFile(filepath.Join(a.dir, "alerte-vazute.json"), b, 0644)
}

type offer struct {
	Store string `json:"store"`
	Name  string `json:"name"`
	Size  string `json:"size"`
	Price *float64
	Disc  any    `json:"disc"`
	From  string `json:"from"`
	To    string `json:"to"`
	Super bool   `json:"super"`
	Q     string `json:"q"`
}

func (o *offer) UnmarshalJSON(b []byte) error {
	type raw offer
	var r struct {
		raw
		P *float64 `json:"price"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*o = offer(r.raw)
	o.Price = r.P
	return nil
}

var mech = regexp.MustCompile(`^(\d+)\+(\d+)$`)

func (o offer) isSuper() bool {
	if o.Price == nil {
		return false
	}
	if o.Super {
		return true
	}
	switch d := o.Disc.(type) {
	case float64:
		return d >= 35
	case string:
		if d == "1+1" {
			return true
		}
		if m := mech.FindStringSubmatch(d); m != nil {
			var x, y float64
			fmt.Sscan(m[1], &x)
			fmt.Sscan(m[2], &y)
			return 100*y/(x+y) >= 35
		}
	}
	return false
}

var storeNames = map[string]string{"lidl": "Lidl", "kaufland": "Kaufland", "carrefour": "Carrefour", "profi": "Profi",
	"dedeman": "Dedeman", "brico": "Brico", "leroy": "Leroy Merlin", "hornbach": "Hornbach"}

// --- potrivire de text (aceeași regulă ca în pagină)
var diac = strings.NewReplacer("ă", "a", "â", "a", "î", "i", "ș", "s", "ş", "s", "ț", "t", "ţ", "t", "Ă", "a", "Â", "a", "Î", "i", "Ș", "s", "Ş", "s", "Ț", "t", "Ţ", "t", "ö", "o", "ü", "u", "é", "e")
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
var sufRe = regexp.MustCompile(`^(|a|e|i|u|ul|ui|ua|aua|uri|urile|ele|ii|ile|ea|le|lor)$`)
var stopW = map[string]bool{"de": true, "cu": true, "si": true, "la": true, "din": true, "pt": true, "pentru": true, "fara": true, "in": true, "un": true, "o": true}
var modW = map[string]bool{"cu": true, "in": true, "fara": true, "si": true, "pentru": true, "la": true}

func words(s string) []string {
	return strings.Fields(nonAlnum.ReplaceAllString(diac.Replace(strings.ToLower(s)), " "))
}
func toks(s string) []string {
	var out []string
	for _, w := range words(s) {
		if len(w) > 1 && !stopW[w] {
			out = append(out, w)
		}
	}
	return out
}
func wmatch(t, w string) bool {
	if strings.HasPrefix(w, t) && sufRe.MatchString(w[len(t):]) {
		return true
	}
	st := t
	if len(t) > 4 {
		st = t[:len(t)-1]
	}
	return strings.HasPrefix(w, st) && sufRe.MatchString(w[len(st):])
}
func goodMatch(q, name string) bool {
	T := toks(q)
	if len(T) == 0 {
		return false
	}
	W := toks(name)
	for _, t := range T {
		ok := false
		for _, w := range W {
			if wmatch(t, w) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	R := words(name)
	for i, w := range R {
		if wmatch(T[0], w) {
			return i <= 2 && (i == 0 || !modW[R[i-1]])
		}
	}
	return false
}

func isBrico(s string) bool { return s == "dedeman" || s == "brico" || s == "leroy" || s == "hornbach" }

func (a *alerts) check() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.set.Notify {
		return
	}
	b := a.online.get()
	if b == nil {
		if lb, err := os.ReadFile(filepath.Join(a.dir, "web", "data.json")); err == nil {
			b = lb
		}
	}
	var d struct {
		Offers []offer `json:"offers"`
	}
	if json.Unmarshal(b, &d) != nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	// ca în pagină: „cafea” nu înseamnă „măsuță de cafea” — păstrăm secțiunea cu mai multe potriviri
	secOK := map[string]bool{}
	for _, w := range a.set.Watch {
		nb, ns := 0, 0
		for _, o := range d.Offers {
			if goodMatch(w, o.Name) || (o.Q != "" && goodMatch(w, o.Q+" "+o.Name)) {
				if isBrico(o.Store) {
					nb++
				} else {
					ns++
				}
			}
		}
		secOK[w] = nb > ns
	}
	var watchHits, superHits []string
	for _, o := range d.Offers {
		if o.To < today || o.From > today {
			continue
		}
		key := o.Store + "|" + o.Name + "|" + o.Size
		line := fmt.Sprintf("%s: %s", storeNames[o.Store], o.Name)
		if o.Price != nil {
			line += fmt.Sprintf(" — %.2f lei", *o.Price)
		}
		matched := false
		for _, w := range a.set.Watch {
			if (goodMatch(w, o.Name) || (o.Q != "" && goodMatch(w, o.Q+" "+o.Name))) && secOK[w] == isBrico(o.Store) {
				matched = true
				break
			}
		}
		if matched && !a.seen["w:"+key] {
			a.seen["w:"+key] = true
			watchHits = append(watchHits, line)
		}
		if a.set.Super && o.isSuper() && !a.seen["s:"+key] {
			a.seen["s:"+key] = true
			superHits = append(superHits, line)
		}
	}
	if len(watchHits) > 0 {
		toast(fmt.Sprintf("🔔 %d oferte la produsele urmărite", len(watchHits)), strings.Join(first(watchHits, 4), "\n"))
	}
	if len(superHits) > 0 {
		toast(fmt.Sprintf("🔥 %d super oferte noi", len(superHits)), strings.Join(first(superHits, 4), "\n"))
	}
	if len(watchHits)+len(superHits) > 0 {
		fmt.Printf("Alerte: %d produse urmarite, %d super oferte noi.\n", len(watchHits), len(superHits))
	}
	a.save()
}

func first(s []string, n int) []string {
	if len(s) > n {
		return append(s[:n:n], fmt.Sprintf("… și încă %d", len(s)-n))
	}
	return s
}

// toast afișează o notificare Windows (fără module suplimentare).
func toast(title, body string) {
	if runtime.GOOS != "windows" {
		fmt.Println("[notificare]", title, "|", body)
		return
	}
	ps := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
$t=[Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$x=$t.GetElementsByTagName('text')
$x.Item(0).AppendChild($t.CreateTextNode($env:OFR_TITLE)) | Out-Null
$x.Item(1).AppendChild($t.CreateTextNode($env:OFR_BODY)) | Out-Null
$n=[Windows.UI.Notifications.ToastNotification]::new($t)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($n)`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.Env = append(os.Environ(), "OFR_TITLE="+title, "OFR_BODY="+body)
	hideWindow(cmd)
	_ = cmd.Run()
}

// --- pornire automată cu Windows (scurtătură în folderul Startup)
func startupLink() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup\OferteRomania.lnk`)
}
func autostartOn() bool { _, err := os.Stat(startupLink()); return err == nil }
func setAutostart(on bool) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("doar pe Windows")
	}
	if !on {
		return os.Remove(startupLink())
	}
	exe, _ := os.Executable()
	ps := `$s=(New-Object -ComObject WScript.Shell).CreateShortcut($env:OFR_LNK);$s.TargetPath=$env:OFR_EXE;$s.Arguments='--fundal';$s.WorkingDirectory=(Split-Path $env:OFR_EXE);$s.WindowStyle=7;$s.Save()`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.Env = append(os.Environ(), "OFR_LNK="+startupLink(), "OFR_EXE="+exe)
	hideWindow(cmd)
	return cmd.Run()
}

func (a *alerts) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		var in struct {
			alertSettings
			Autostart *bool `json:"autostart"`
			Test      bool  `json:"test"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "date invalide", 400)
			return
		}
		if in.Test {
			go toast("🔔 Oferte România", "Notificările sunt pornite. Așa vei vedea ofertele noi.")
		}
		var aerr string
		if in.Autostart != nil && *in.Autostart != autostartOn() {
			if err := setAutostart(*in.Autostart); err != nil {
				aerr = err.Error()
			}
		}
		a.mu.Lock()
		changed := strings.Join(a.set.Watch, "|") != strings.Join(in.Watch, "|") || a.set.Notify != in.Notify || a.set.Super != in.Super
		a.set = in.alertSettings
		a.mu.Unlock()
		if changed {
			a.mu.Lock()
			a.save()
			a.mu.Unlock()
			go a.check()
		}
		fmt.Fprintf(w, `{"ok":true,"autostart":%v,"err":%q}`, autostartOn(), aerr)
		return
	}
	a.mu.Lock()
	b, _ := json.Marshal(struct {
		alertSettings
		Autostart bool `json:"autostart"`
	}{a.set, autostartOn()})
	a.mu.Unlock()
	w.Write(b)
}
