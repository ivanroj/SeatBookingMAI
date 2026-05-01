#!/usr/bin/env bash
# End-to-end smoke test against the running stack started via docker compose.
# Run from the project root: ./scripts/smoke.sh
set -euo pipefail

BACKEND="${BACKEND_URL:-http://127.0.0.1:8080}"
FRONTEND="${FRONTEND_URL:-http://127.0.0.1:3000}"

echo "==> /healthz on backend"
curl -fsS "$BACKEND/healthz" && echo

echo "==> Frontend serves index.html"
curl -fsS -o /dev/null -w "  HTTP %{http_code}\n" "$FRONTEND/"

echo "==> Login as seed admin"
TOKEN=$(curl -fsS -X POST "$BACKEND/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@mai.ru","password":"admin123"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
echo "  token=${TOKEN:0:8}…"

auth=(-H "Authorization: Bearer $TOKEN")

echo "==> /api/auth/me"
curl -fsS "${auth[@]}" "$BACKEND/api/auth/me" && echo

echo "==> Pick a free seat 2h from now"
START=$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=2)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
END=$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=3)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
echo "  window: $START .. $END"
SEATS=$(curl -fsS "${auth[@]}" "$BACKEND/api/seats/available?start_at=$START&end_at=$END")
echo "  available: $SEATS"
SEAT_ID=$(echo "$SEATS" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])')

echo "==> Create booking on seat $SEAT_ID"
BOOKING=$(curl -fsS -X POST "${auth[@]}" -H 'Content-Type: application/json' \
  -d "{\"seat_id\":$SEAT_ID,\"start_at\":\"$START\",\"end_at\":\"$END\"}" \
  "$BACKEND/api/bookings")
echo "  $BOOKING"
BOOKING_ID=$(echo "$BOOKING" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

echo "==> List my bookings"
curl -fsS "${auth[@]}" "$BACKEND/api/bookings/me" && echo

echo "==> Cancel booking $BOOKING_ID"
curl -fsS -X DELETE "${auth[@]}" -o /dev/null -w "  HTTP %{http_code}\n" "$BACKEND/api/bookings/$BOOKING_ID"

echo "==> Admin reports for next 24h"
FROM=$(python3 -c 'import datetime; print(datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))')
TO=$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
curl -fsS "${auth[@]}" "$BACKEND/api/admin/reports?from=$FROM&to=$TO" && echo

echo "==> All good."
