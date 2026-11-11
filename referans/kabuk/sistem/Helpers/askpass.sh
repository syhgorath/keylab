#!/usr/bin/env bash
# askpass.sh - ssh-add için SSH_ASKPASS yardımcısı
# Passphrase'i Keychain'den okuyup stdout'a yazar; diske passphrase yazılmaz.
# ssh-add yanlış passphrase'te boş cevap alana kadar SONSUZ yeniden dener; bu yüzden
# ASKPASS_STATE verilmişse yalnızca İLK çağrıda passphrase döner, sonrakilerde boş
# satır döner (ssh-add vazgeçer).
if [ -n "${ASKPASS_STATE:-}" ]; then
    if [ -e "$ASKPASS_STATE" ]; then echo ""; exit 0; fi
    : > "$ASKPASS_STATE"
fi
exec security find-generic-password -a "${ASKPASS_ACCOUNT:?}" -s "${ASKPASS_SERVICE:-USBMonitor}" -w
