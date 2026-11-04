#!/usr/bin/env bash
# Menu.sh - Menü gösterimi ve seçim sistemi (bash 3.2+ uyumlu)

# Display a menu for a group and return selected module function name
# Parameters:
#   $1 - Group name (e.g., "MainMenu")
# Returns: Selected module function name via echo
Menu() {
  local group="$1"
  
  if [ -z "$group" ]; then
    echo "Error: Menu requires a group name" >&2
    return 1
  fi
  
  # Get modules in group
  local modules
  modules=$(GetModulesInGroup "$group")
  
  if [ -z "$modules" ]; then
    echo "Error: No modules found in group: $group" >&2
    return 1
  fi
  
  # Convert space-separated to array (bash 3.2 compatible)
  local module_array
  local count=0
  local i=1
  
  # Count modules and build array
  # IFS'i geçici olarak space'e ayarla (bash 3.2 uyumlu)
  local OLD_IFS="$IFS"
  IFS=' '
  # shellcheck disable=SC2086  # kasıtlı: IFS=' ' ile kelimelere bölünür
  set -- $modules
  for module; do
    # Boş modül adlarını atla
    [ -z "$module" ] && continue
    module_array[i]="$module"
    ((i++))
    ((count++))
  done
  IFS="$OLD_IFS"
  
  # Display menu (stderr'e yaz, böylece stdout sadece seçimi içerir)
  if declare -F ui_header >/dev/null; then
    ui_header "$(ui_group_title "$group")" 2 >&2
  else
    echo "" >&2
    echo "=== $group ===" >&2
  fi
  # Durum ayarı açıksa (varsayılan; bkz. Settings.sh), <modül>_status fonksiyonu olanların durumu paralel kontrol edilir
  local status_dir=""
  if declare -F settings_get >/dev/null && [ "$(settings_get STATUS)" = "on" ]; then
    status_dir=$(mktemp -d)
    i=1
    while [ $i -le $count ]; do
      case "${module_array[$i]}" in
        server:*)
          server_status "${module_array[$i]#server:}" > "$status_dir/$i" 2>/dev/null &
          ;;
        *)
          if declare -F "${module_array[$i]}_status" >/dev/null 2>&1; then
            "${module_array[$i]}_status" > "$status_dir/$i" 2>/dev/null &
          fi
          ;;
      esac
      ((i++))
    done
    wait
  fi
  local row_text=() row_mark=()
  i=1
  while [ $i -le $count ]; do
    local func_name="${module_array[$i]}"
    local label
    label=$(GetModuleLabel "$func_name")
    if [ -z "$label" ]; then
      label="$func_name"
    fi
    local mark=""
    if [ -n "$status_dir" ] && [ -f "$status_dir/$i" ]; then
      mark="$(cat "$status_dir/$i") "
      row_mark[i - 1]="${mark% }"
    fi
    local detail=""
    case "$func_name" in
      server:*) detail=$(server_summary "${func_name#server:}" 2>/dev/null) ;;
    esac
    if [ -n "$detail" ]; then
      row_text[i - 1]="${mark}${label}   ${detail}"
    else
      row_text[i - 1]="${mark}${label}"
    fi
    ((i++))
  done
  if [ -n "$status_dir" ]; then rm -r "$status_dir"; fi

  # Ok tuşlu seçim (terminalde); aksi halde numaralı menü
  if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; then
    local picked
    local -a UI_PICK_DESCS=()
    local di=0
    while [ "$di" -lt "$count" ]; do
      # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
      UI_PICK_DESCS[di]=$(ui_describe_entry "${module_array[$((di + 1))]}" "${row_mark[$di]:-}")
      di=$((di + 1))
    done
    # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
    local UI_PICK_BACK_DESC="Uygulamadan çıkar (USB çıkarma denenir)."
    picked=$(ui_pick_arrows 1 "Çıkış" "${row_text[@]}")
    if [ -z "$picked" ] || [ "$picked" = "0" ]; then
      echo -n "exit"
    else
      echo -n "${module_array[$picked]}"
    fi
    return 0
  fi

  i=1
  while [ $i -le $count ]; do
    printf "  %2d) %s\n" "$i" "${row_text[$((i - 1))]}" >&2
    ((i++))
  done
  echo "  0) Çıkış (q)" >&2
  if declare -F ui_footer >/dev/null; then
    ui_footer "numara yaz + Enter · 0 veya q = çıkış" 2 >&2
  else
    echo "" >&2
  fi
  
  # Get user selection
  while true; do
    # Girdi biterse (EOF/pipe kapandı) sonsuz döngüye girmeden çık
    if ! read -r -p "Seçim [0-$count, q]: " selection; then
      echo -n "exit"
      return 0
    fi
    
    # Trim whitespace
    selection=$(echo "$selection" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    
    # q/Q = 0 (geri/çıkış)
    case "$selection" in q|Q) selection=0 ;; esac

    # Validate input
    if [[ "$selection" =~ ^[0-9]+$ ]]; then
      if [ "$selection" -eq 0 ]; then
        echo -n "exit"
        return 0
      elif [ "$selection" -ge 1 ] && [ "$selection" -le "$count" ]; then
        local selected_module="${module_array[$selection]}"
        if [ -n "$selected_module" ]; then
          echo -n "$selected_module"
          return 0
        else
          echo "Error: Module not found for selection: $selection" >&2
        fi
      else
        echo "Invalid selection. Please enter a number between 0 and $count." >&2
      fi
    else
      echo "Invalid selection. Please enter a number between 0 and $count." >&2
    fi
  done
}
