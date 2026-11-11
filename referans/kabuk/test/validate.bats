#!/usr/bin/env bats
load test_helper
setup() { load_helpers; }

@test "geçerli hedef kabul edilir" {
    run ssh_validate_target "192.0.2.10" "pi" "22"
    [ "$status" -eq 0 ]
}
@test "IPv6 ve host adı kabul edilir" {
    run ssh_validate_target "fe80::1" "pi" "22";           [ "$status" -eq 0 ]
    run ssh_validate_target "my-host.example.com" "u_1" "2222"; [ "$status" -eq 0 ]
}
@test "kullanıcıda ; ve boşluk reddedilir" {
    run ssh_validate_target "10.0.0.1" "pi;id" "22";   [ "$status" -ne 0 ]
    run ssh_validate_target "10.0.0.1" "a b" "22";     [ "$status" -ne 0 ]
}
@test "kullanıcı '-' ile başlayamaz (seçenek enjeksiyonu)" {
    run ssh_validate_target "10.0.0.1" "-oProxyCommand=x" "22"; [ "$status" -ne 0 ]
    run ssh_validate_target "10.0.0.1" "-v" "22";               [ "$status" -ne 0 ]
}
@test "host'ta tehlikeli karakterler reddedilir" {
    run ssh_validate_target '10.0.0.1$(id)' "pi" "22"; [ "$status" -ne 0 ]
    run ssh_validate_target "a b" "pi" "22";           [ "$status" -ne 0 ]
}
@test "port sınırları" {
    run ssh_validate_target "h" "u" "0";     [ "$status" -ne 0 ]
    run ssh_validate_target "h" "u" "65536"; [ "$status" -ne 0 ]
    run ssh_validate_target "h" "u" "abc";   [ "$status" -ne 0 ]
    run ssh_validate_target "h" "u" "1";     [ "$status" -eq 0 ]
    run ssh_validate_target "h" "u" "65535"; [ "$status" -eq 0 ]
}
@test "modül adı kuralları" {
    run module_name_valid "myserver"; [ "$status" -eq 0 ]
    run module_name_valid "web_01";   [ "$status" -eq 0 ]
    run module_name_valid "1web";     [ "$status" -ne 0 ]
    run module_name_valid "Web";      [ "$status" -ne 0 ]
    run module_name_valid "a-b";      [ "$status" -ne 0 ]
    run module_name_valid "";         [ "$status" -ne 0 ]
}
