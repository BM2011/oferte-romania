"""Trimite pe ntfy.sh super ofertele NOI (comparând data.json cu versiunea anterioară).
Abonare: aplicația ntfy (Android / iPhone) -> subiectul de mai jos."""
import json, os, re, subprocess, sys, urllib.parse, urllib.request
from datetime import date

TOPIC = os.environ.get("NTFY_TOPIC", "oferte-romania-bm2011")
APP = "https://bm2011.github.io/oferte-romania/"
NAMES = {"lidl": "Lidl", "kaufland": "Kaufland", "carrefour": "Carrefour", "profi": "Profi",
         "dedeman": "Dedeman", "brico": "Brico", "leroy": "Leroy Merlin", "hornbach": "Hornbach"}
BRICO = {"dedeman", "brico", "leroy", "hornbach"}

def load(ref):
    try:
        out = subprocess.run(["git", "show", f"{ref}:web/data.json"], capture_output=True, check=True).stdout
        return json.loads(out)
    except Exception:
        return {"offers": []}

def dnum(o):
    d = o.get("disc")
    if isinstance(d, (int, float)): return d
    m = re.fullmatch(r"(\d+)\+(\d+)", str(d or ""))
    return round(100 * int(m[2]) / (int(m[1]) + int(m[2]))) if m else 0

def is_super(o):
    return o.get("price") is not None and (o.get("super") is True or dnum(o) >= 35 or o.get("disc") == "1+1")

key = lambda o: f'{o["store"]}|{o["name"]}|{o.get("size","")}'
new, old = load("HEAD"), load("HEAD~1")
today = date.today().isoformat()
before = {key(o) for o in old.get("offers", []) if is_super(o)}
fresh = [o for o in new.get("offers", []) if is_super(o) and key(o) not in before and o.get("to", "") >= today]
if not fresh:
    print("Nicio super ofertă nouă."); sys.exit(0)

def line(o):
    d = f' (−{o["disc"]}%)' if isinstance(o.get("disc"), (int, float)) else (f' ({o["disc"]})' if o.get("disc") else "")
    price = f'{o["price"]:.2f}'.replace(".", ",")
    return f'• {NAMES.get(o["store"], o["store"])}: {o["name"]} — {price} lei{d}'

for sec, title, tag in (("super", "🛒 Super oferte noi la supermarket", "shopping_cart"), ("brico", "🔨 Super oferte noi la bricolaj", "hammer")):
    L = [o for o in fresh if (o["store"] in BRICO) == (sec == "brico")]
    if not L: continue
    L.sort(key=lambda o: (-(o.get("super") is True), -dnum(o)))
    body = "\n".join(line(o) for o in L[:8]) + (f"\n… și încă {len(L) - 8}" if len(L) > 8 else "")
    q = urllib.parse.urlencode({"title": f"{title} ({len(L)})", "tags": tag, "click": APP + ("#bricolaj" if sec == "brico" else "")})
    req = urllib.request.Request(f"https://ntfy.sh/{TOPIC}?{q}", data=body.encode("utf-8"), method="POST",
                                 headers={"Content-Type": "text/plain; charset=utf-8"})
    with urllib.request.urlopen(req, timeout=20) as r:
        print(sec, r.status, len(L))
