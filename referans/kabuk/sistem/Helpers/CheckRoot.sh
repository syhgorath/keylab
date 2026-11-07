#!/usr/bin/env bash
# CheckRoot.sh - Root kontrolü (USB için opsiyonel)

# Check if running as root
# Parameters: None
# Returns: 0 if root, 1 if not root
CheckRoot() {
  if [ "$EUID" -eq 0 ]; then
    return 0
  else
    return 1
  fi
}

# Check root and exit if not root (optional for USB)
# Parameters: None
CheckRootOrExit() {
  if ! CheckRoot; then
    echo "This script requires root privileges."
    echo "Note: For USB autorun, root is usually not required."
    exit 1
  fi
}

