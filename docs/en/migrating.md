# Migrating from the old layout (pre-v3 → `servers/`)

Home: [README](../../README.en.md).

Your old `sistem/Modules/<name>/` servers are migrated **automatically**; no export/import needed.

```bash
./start.sh --migrate --dry-run   # see what would happen (changes nothing)
./start.sh --migrate             # asks for confirmation, then migrates
```

Keys are copied and verified, old folders are **not deleted** (`servers/.migrated-<date>/`). Keychain passphrases stay valid.
Once you have verified everything works, delete that folder yourself (it holds copies of the keys).
