# Oferte România

Aplicație web care adună într-un singur loc promoțiile din cataloagele magazinelor din România:

- **🛒 Supermarket:** Lidl, Kaufland, Carrefour, Profi
- **🔨 Bricolaj:** Dedeman, Brico, Leroy Merlin, Hornbach

Funcții: ofertele sunt grupate pe categorii de produse, fiecare secțiune începe cu un rând de **🔥 Super oferte** (reduceri de cel puțin 35% sau 1+1). Poți filtra după magazin, căuta un produs, sorta după reducere, preț sau data expirării, ascunde ofertele care cer card de fidelitate și îți poți face o **listă de cumpărături** cu total estimat.

## Rulare locală (Windows)

Pornește `OferteRomania.exe`. Executabilul deschide un server local (http://127.0.0.1:8787) și pornește aplicația în browser. Închide fereastra neagră ca să oprești aplicația.

La prima pornire, executabilul caută fotografiile produselor de marcă în [Open Food Facts](https://world.openfoodfacts.org) (baza de date deschisă, imagini sub licență CC BY-SA). Durează câteva minute, apoi rezultatele rămân salvate în `images-cache.json`. Produsele fără fotografie (fructe, carne vrac, scule) păstrează o iconiță.

La pornire, executabilul descarcă și cea mai nouă listă de oferte din acest repository (`web/data.json`, actualizat zilnic). Fără internet, folosește ultima listă salvată sau ofertele incluse.

Dacă lângă executabil există folderul `web/`, aplicația folosește fișierele de acolo. Poți actualiza ofertele înlocuind doar `web/data.json`, fără să refaci executabilul.

## Pe telefon

Aplicația se adaptează automat la ecranul telefonului (2 carduri pe rând, meniuri glisante).

1. Pornește `OferteRomania.exe` pe calculator. Dacă Windows întreabă, permite accesul pentru **rețele private**.
2. Apasă **„📱 Deschide pe telefon”** în pagină și scanează codul QR cu telefonul (telefonul trebuie să fie în aceeași rețea Wi-Fi).
3. Pe telefon, din meniul browserului, alege **„Adaugă pe ecranul de pornire”** ca s-o ai ca aplicație.

## Structură

```
web/index.html   interfața (un singur fișier HTML/CSS/JS, fără dependențe)
web/data.json    ofertele, campaniile și sursele
web/manifest.json, icon-*.png   instalarea ca aplicație pe telefon
web/qrcode.js    generator cod QR (qrcode-generator, licență MIT)
main.go          serverul local care produce executabilul
photos.go        căutarea fotografiilor în Open Food Facts
online.go        descărcarea ofertelor actualizate din GitHub
build.bat        recompilează executabilul pe Windows (necesită Go)
```

## Recompilare

Cu [Go](https://go.dev/dl/) 1.22+ instalat:

```
go build -trimpath -ldflags "-s -w" -o OferteRomania.exe .
```

## Formatul datelor

Fiecare ofertă din `data.json`:

```json
{"store":"lidl","name":"Cașcaval Pilos","size":"300 g","price":8.99,"old":13.09,"disc":31,
 "from":"2026-11-09","to":"2026-11-15","cat":"lactate","app":true,"calc":false,"unit":"","super":false}
```

Chei magazin: `lidl`, `kaufland`, `carrefour`, `profi`, `dedeman`, `brico`, `leroy`, `hornbach`.

## Notă

Proiect personal, neafiliat magazinelor menționate. Prețurile provin din cataloage și articole de presă publice și pot varia între magazine; ofertele sunt valabile în limita stocului. Verifică întotdeauna catalogul oficial înainte de cumpărare. Fotografiile de produs provin din Open Food Facts (CC BY-SA); nu folosim imagini din cataloagele magazinelor.
