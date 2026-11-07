#!/usr/bin/env bash
# Log.sh - Log fonksiyonu

# Log levels (macOS uyumluluğu için -g olmadan)
LOG_LEVEL_INFO="INFO"
LOG_LEVEL_WARN="WARN"
LOG_LEVEL_ERROR="ERROR"
LOG_LEVEL_DEBUG="DEBUG"
export LOG_LEVEL_INFO LOG_LEVEL_WARN LOG_LEVEL_ERROR LOG_LEVEL_DEBUG

# Log a message with timestamp
# Parameters:
#   $1 - Log level (INFO, WARN, ERROR, DEBUG)
#   $2 - Message
Log() {
  local level="${1:-INFO}"
  local message="$2"
  local timestamp
  timestamp=$(date '+%Y-%m-%d %H:%M:%S')
  
  if [ -z "$message" ]; then
    echo "Error: Log requires a message" >&2
    return 1
  fi
  
  # Color codes (if terminal supports it)
  local color_reset="\033[0m"
  local color_info="\033[0;32m"   # Green
  local color_warn="\033[0;33m"   # Yellow
  local color_error="\033[0;31m"  # Red
  local color_debug="\033[0;36m"  # Cyan
  
  local color="$color_reset"
  case "$level" in
    INFO)
      color="$color_info"
      ;;
    WARN)
      color="$color_warn"
      ;;
    ERROR)
      color="$color_error"
      ;;
    DEBUG)
      color="$color_debug"
      ;;
  esac
  
  # Output to stderr for errors, stdout for others
  if [ "$level" = "ERROR" ]; then
    echo -e "${color}[$timestamp] [$level] $message${color_reset}" >&2
  else
    echo -e "${color}[$timestamp] [$level] $message${color_reset}"
  fi
  
  # Also write to log file if USB_LOG_FILE is set
  if [ -n "${USB_LOG_FILE:-}" ] && [ -d "$(dirname "$USB_LOG_FILE")" ]; then
    echo "[$timestamp] [$level] $message" >> "$USB_LOG_FILE"
  fi
}

