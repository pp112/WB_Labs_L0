#!/bin/bash

# Usage: ./send_order.sh /path/to/file_or_folder

if [ $# -lt 1 ]; then
  echo "Usage: $0 <file_or_directory_path>"
  exit 1
fi

PATH_TO_SEND=$1

docker compose exec publisher go run /app/publish.go "$PATH_TO_SEND"
