#!/bin/sh

set -eu

host="${1:-http://127.0.0.1:8080}"
chat_id="${2:-1396374322}"

curl -fsS -X POST "${host}/webhook/telegram" \
  -H 'content-type: application/json' \
  -d "{
    \"update_id\": 1,
    \"message\": {
      \"message_id\": 1,
      \"from\": {
        \"id\": ${chat_id},
        \"first_name\": \"Lab\",
        \"username\": \"lab-user\"
      },
      \"chat\": {
        \"id\": ${chat_id},
        \"type\": \"private\"
      },
      \"text\": \"investigate payments-api\",
      \"date\": 1775260800
    }
  }"
