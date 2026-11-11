#!/usr/bin/env bash
# RegisterModules.sh - Modül kayıt sistemi (bash 3.2+ uyumlu)

# Bash 3.2 uyumlu modül kayıt sistemi
# Associative array yerine string-based sistem kullanıyoruz

# Modül kayıt formatı: "function_name|label|group"
# Global değişkenler
MODULE_REGISTRY=""
MODULE_COUNT=0

# Register a module with a label and group
# Parameters:
#   $1 - Module label (display name)
#   $2 - Module group (e.g., "MainMenu", "SecurityMenu")
#   $3 - Module function name (optional, defaults to module label)
RegisterModules() {
  local label="$1"
  local group="$2"
  local function_name="${3:-$label}"
  
  if [ -z "$label" ] || [ -z "$group" ]; then
    echo "Error: RegisterModules requires label and group" >&2
    return 1
  fi
  
  # Aynı etiket ve grup zaten kayıtlıysa (örn. aynı adlı eski modül) yinelenmesin; ilk kayıt kazanır
  if [ -n "$MODULE_REGISTRY" ] && printf '%s\n' "$MODULE_REGISTRY" \
      | awk -F'|' -v l="$label" -v g="$group" '$2==l && $3==g {f=1} END{exit !f}'; then
    return 0
  fi

  # Modülü kaydet: "function_name|label|group"
  local entry="$function_name|$label|$group"
  
  if [ -z "$MODULE_REGISTRY" ]; then
    MODULE_REGISTRY="$entry"
  else
    # Yeni satır ekle (newline ile)
    MODULE_REGISTRY="${MODULE_REGISTRY}
${entry}"
  fi
  
  MODULE_COUNT=$((MODULE_COUNT + 1))
  
  return 0
}

# Get module label by function name
# Parameters:
#   $1 - Function name
# Returns: Label via echo
GetModuleLabel() {
  local func_name="$1"
  echo "$MODULE_REGISTRY" | grep "^$func_name|" | cut -d'|' -f2
}

# Get module group by function name
# Parameters:
#   $1 - Function name
# Returns: Group via echo
GetModuleGroup() {
  local func_name="$1"
  echo "$MODULE_REGISTRY" | grep "^$func_name|" | cut -d'|' -f3
}

# Get all modules in a group
# Parameters:
#   $1 - Group name
# Returns: Function names (space-separated) via echo
GetModulesInGroup() {
  local group="$1"
  if [ -z "$MODULE_REGISTRY" ] || [ -z "$group" ]; then
    return 0
  fi
  # Her satırı kontrol et ve group ile eşleşenleri bul
  # awk kullanarak daha güvenilir parsing
  # Her modülü ayrı satırda yazdır, sonra space-separated yap
  echo "$MODULE_REGISTRY" | awk -F'|' -v group="$group" '
    NF == 3 {
      gsub(/^[ \t]+|[ \t]+$/, "", $3)  # Trim whitespace from group
      if ($3 == group) {
        gsub(/^[ \t]+|[ \t]+$/, "", $1)  # Trim whitespace from function name
        print $1
      }
    }
  ' | grep -v '^[[:space:]]*$' | tr '\n' ' ' | sed 's/[[:space:]]*$//'
}

# Check if module exists
# Parameters:
#   $1 - Function name
# Returns: 0 if exists, 1 if not
ModuleExists() {
  local func_name="$1"
  echo "$MODULE_REGISTRY" | grep -q "^$func_name|"
}
