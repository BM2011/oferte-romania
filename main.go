// Oferte România — server local pentru testarea aplicației.
// Pornește un server web pe calculatorul tău și deschide aplicația în browser.
// Dacă lângă executabil există un folder "web" (index.html, data.json),
// fișierele de acolo au prioritate față de cele incluse în executabil.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed web
var embedded embed.FS

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func main() {
	var root fs.FS
	sursa := "fisierele incluse in executabil"
	exeDir := "."
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
		local := filepath.Join(exeDir, "web")
		if st, err := os.Stat(filepath.Join(local, "index.html")); err == nil && !st.IsDir() {
			root = os.DirFS(local)
			sursa = local
		}
	}
	if root == nil {
		sub, _ := fs.Sub(embedded, "web")
		root = sub
	}

	// Ascultă pe toate interfețele ca aplicația să poată fi deschisă și de pe telefon (aceeași rețea Wi-Fi).
	ln, err := net.Listen("tcp", "0.0.0.0:8787")
	if err != nil {
		ln, err = net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			fmt.Println("Nu pot porni serverul:", err)
			fmt.Println("Apasa Enter pentru a inchide.")
			fmt.Scanln()
			return
		}
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	lan := ""
	if ip := lanIP(); ip != "" {
		lan = fmt.Sprintf("http://%s:%d/", ip, port)
	}

	mux := http.NewServeMux()
	files := http.FileServer(http.FS(root))
	photos := newPhotoCache(exeDir)
	go func() { time.Sleep(1500 * time.Millisecond); photos.prefetch(root) }()
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/api/img", photos.handler)
	// Ofertele cele mai noi: data.json din GitHub (actualizat zilnic de bot), cu rezervă locală.
	online := newOnlineData(exeDir)
	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		if b := online.get(); b != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(b)
			return
		}
		files.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"lan":%q}`, lan)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})

	fmt.Println("=============================================")
	fmt.Println("  Oferte Romania — versiune de test")
	fmt.Println("=============================================")
	fmt.Println("Pe acest calculator:", url)
	if lan != "" {
		fmt.Println("Pe telefon (aceeasi retea Wi-Fi):", lan)
		fmt.Println("  (sau apasa butonul \"Deschide pe telefon\" din pagina pentru cod QR)")
		fmt.Println("  Daca Windows intreaba, permite accesul pentru retele PRIVATE.")
	}
	fmt.Println("Date incarcate din:", sursa)
	fmt.Println()
	fmt.Println("Inchide aceasta fereastra pentru a opri aplicatia.")
	fmt.Println()

	go func() { time.Sleep(400 * time.Millisecond); openBrowser(url) }()
	if err := http.Serve(ln, mux); err != nil {
		fmt.Println("Eroare server:", err)
		fmt.Scanln()
	}
}

// lanIP întoarce adresa IPv4 din rețeaua locală (ex. 192.168.x.x).
func lanIP() string {
	// Metoda sigură: interfața folosită pentru ieșirea spre internet (nu trimite date).
	if c, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer c.Close()
		if a, ok := c.LocalAddr().(*net.UDPAddr); ok && a.IP.IsPrivate() {
			return a.IP.String()
		}
	}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
			return n.IP.String()
		}
	}
	return ""
}
