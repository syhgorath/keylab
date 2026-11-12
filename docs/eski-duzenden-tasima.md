# Eski düzenden taşıma (v3 öncesi → `servers/`)

Ana sayfa: [README](../README.md).

> **Not:** Bu belge eski kabuk sürümü (v3) içindir. Sürüm 4'te (Go) `--migrate` yoktur; eski motor `kabuk-referansi` etiketinde durur.

Eski `sistem/Modules/<ad>/` sunucularınız **otomatik** taşınır; dışa/içe aktarma gerekmez.

```bash
./start.sh --migrate --dry-run   # önce ne olacağını gör (hiçbir şey değişmez)
./start.sh --migrate             # onay isteyip taşır
```

Key'ler kopyalanıp doğrulanır, eski klasörler **silinmez** (`servers/.migrated-<tarih>/`). Keychain'deki passphrase'ler aynen geçerlidir.
Doğruladıktan sonra o klasörü (key kopyaları içerir) kendiniz silin.
