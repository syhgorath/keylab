#!/usr/bin/env bash
# ci-install-shellcheck.sh - CI için SABİT sürüm ShellCheck kurar (apt/brew'in "güncel"i yerel ile CI'yı ayırıyordu).
# İndirilen arşivin sha256'sı aşağıdaki tabloyla eşleşmezse HİÇBİR ŞEY kurulmaz.
# Özetler GitHub'ın sürüm API'sinden (assets[].digest) alınmıştır: koalaman/shellcheck v0.11.0.
#
# Kullanım: bash scripts/ci-install-shellcheck.sh [hedef-dizin]   (varsayılan: ~/.local/bin; GITHUB_PATH varsa eklenir)
# Test için: SC_LOCAL_ARCHIVE=<yol> indirmeyi atlar (özet yine doğrulanır); SC_PRINT_ONLY=1 yalnızca varlık+özeti yazar.
set -euo pipefail

VERSION="0.11.0"
case "$(uname -s)-$(uname -m)" in
    Linux-x86_64)  asset="linux.x86_64";   sha="8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198" ;;
    Darwin-arm64)  asset="darwin.aarch64"; sha="56affdd8de5527894dca6dc3d7e0a99a873b0f004d7aabc30ae407d3f48b0a79" ;;
    Darwin-x86_64) asset="darwin.x86_64";  sha="3c89db4edcab7cf1c27bff178882e0f6f27f7afdf54e859fa041fca10febe4c6" ;;
    *) echo "desteklenmeyen platform: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac
if [ "${SC_PRINT_ONLY:-0}" = "1" ]; then echo "$asset $sha"; exit 0; fi

dest="${1:-$HOME/.local/bin}"
tmp=$(mktemp -d)
trap 'rm -r -- "${tmp:?}"' EXIT
archive="$tmp/shellcheck.tar.xz"

if [ -n "${SC_LOCAL_ARCHIVE:-}" ]; then
    cp "$SC_LOCAL_ARCHIVE" "$archive"
else
    url="https://github.com/koalaman/shellcheck/releases/download/v${VERSION}/shellcheck-v${VERSION}.${asset}.tar.xz"
    curl -fsSL --proto '=https' --tlsv1.2 --max-time 120 -o "$archive" "$url"
fi

if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$archive" | cut -d' ' -f1)
else got=$(shasum -a 256 "$archive" | cut -d' ' -f1); fi
if [ "$got" != "$sha" ]; then
    echo "HATA: ShellCheck arşivinin sha256'sı beklenenle uyuşmuyor ($got); kurulmadı." >&2
    exit 1
fi

tar -xJf "$archive" -C "$tmp"
mkdir -p "$dest"
install -m 0755 "$tmp/shellcheck-v${VERSION}/shellcheck" "$dest/shellcheck"
if [ -n "${GITHUB_PATH:-}" ]; then echo "$dest" >> "$GITHUB_PATH"; fi
"$dest/shellcheck" --version | sed -n 2p
