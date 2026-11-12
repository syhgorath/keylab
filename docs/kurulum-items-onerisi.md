# Öneri: `kurulum` paketine `items` bildirimi (0.1.1)

Arayüzün genel katalog sayfası (bkz. `docs/package-standard.md`, "Katalog sayfası") `kurulum` için şimdilik yerleşik bir listeyle çalışır.
Paket `items` bildirince yerleşik liste devre dışı kalır ve kartlar manifestten gelir. Aşağıdaki alanlar `component.json`'a **üst düzeyde** eklenir
(`vars`/`actions` değişmez, `sshpocket_api` artmaz). Bu blok kurulum 0.1.0'ın kopyasında `keylab components validate` ile denendi: **✅ uyumlu**.

```json
"item_var": "kurulum_yazilim",
"mode_var": "kurulum_mod",
"items": [
  {"id": "docker",  "label": "Docker Engine", "description": "Konteyner çalışma ortamı ve Docker Compose eklentisi. Konteyner modundaki uygulamalar için gereklidir.",
   "modes": ["native"], "actions": ["preview", "kur", "yukselt", "kaldir"], "params": ["kurulum_docker_kaynak"]},
  {"id": "nginx",   "label": "Nginx",   "description": "HTTP ters proxy ve statik dosya sunucusu.",
   "modes": ["native", "docker"], "actions": ["preview", "kur", "yukselt", "kaldir"], "params": ["kurulum_nginx_port", "kurulum_nginx_surum", "kurulum_nginx_imaj_ozel"]},
  {"id": "mariadb", "label": "MariaDB", "description": "MySQL uyumlu ilişkisel SQL veritabanı sunucusu.",
   "modes": ["native", "docker"], "actions": ["preview", "kur", "yukselt", "kaldir"], "params": ["kurulum_mariadb_port", "kurulum_mariadb_surum", "kurulum_mariadb_imaj_ozel"]},
  {"id": "redis",   "label": "Redis",   "description": "Anahtar-değer veri önbelleği ve mesaj kuyruğu.",
   "modes": ["native", "docker"], "actions": ["preview", "kur", "yukselt", "kaldir"], "params": ["kurulum_redis_port", "kurulum_redis_surum", "kurulum_redis_imaj_ozel"]}
]
```

Notlar
- `item_var`/`mode_var` mevcut `string`/`enum` değişkenlerdir (`kurulum_yazilim`: docker, nginx, mariadb, redis; `kurulum_mod`: native, docker); öğe kimlikleri ve modları bu enum değerleriyle eşleşmelidir.
- `actions` listesindeki her eylemin `requires` ön koşulu da listede olmalıdır (`kur`/`yukselt`/`kaldir` → `preview`).
- `durum` eylemi zaten `kurulu` ve `portlar` anahtarlarını bildiriyor; arayüz durum rozetlerini bunlardan okur.
- Bu bir paket sürümü değişikliğidir (kurulum deposu, imza, etiket, indeks): ilgili adımlar bakımcıdadır.
