#!/usr/bin/env bash
#set -euo pipefail
IFS=$'\n\t'
shopt -s nullglob

# Recursively load all shell scripts from a directory tree
# This function discovers and sources all .sh files in the given path and subdirectories,
# enabling automatic module registration. Skips symbolic links to avoid loops.
# Parameters:
#   $1 - Directory path to search for modules
# Returns: None (but may output warnings to stderr)
# Side effects: Sources all found shell scripts, which may register modules and define functions
# Example: Autoload "$USB_PATH/sistem"
Autoload() {
  local path="$1"
  
  # Path kontrolü
  if [ ! -d "$path" ]; then
    echo "Warning: Autoload path does not exist: $path" >&2
    return 1
  fi
  
  # Iterate through all items in the directory
  for item in "$path"/*; do
    # Skip if no files found (nullglob)
    [ ! -e "$item" ] && continue
    
    # Skip symbolic links to prevent infinite loops
    [ -L "$item" ] && continue
    
    # If item is a file, check if it's a shell script (.sh extension)
    if [ -f "$item" ]; then
      local basename_item
      basename_item=$(basename "$item")
      
      # Skip Autoload.sh and Main.sh to avoid recursion
      [ "$basename_item" = "Autoload.sh" ] && continue
      [ "$basename_item" = "Main.sh" ] && continue
      
      # Only load .sh files (shell scripts)
      # Skip SSH keys, .env files, images, and other non-script files
      if [[ ! "$basename_item" =~ \.sh$ ]]; then
        continue
      fi
      
      echo "Loading module: $item"
      # Source the file and capture errors; continue on failure to allow other modules to load
      # shellcheck disable=SC1090  # modül yolları çalışma anında belirlenir
      if ! source "$item" 2>&1; then
        echo "Warning: Failed to load module: $item" >&2
        continue  # Continue loading other modules even if one fails
      fi
    # If item is a directory, recursively load modules from it
    elif [ -d "$item" ]; then
      Autoload "$item"
    fi
  done
}

