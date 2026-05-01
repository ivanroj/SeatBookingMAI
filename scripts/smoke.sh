#!/usr/bin/env bash
# End-to-end smoke test against the running stack started via docker compose.
# Exercises both flows defined by the spec:
#   * Student (UC-2/3/4) — anonymous, device-bound.
#   * Admin   (UC-1 + UC-5/6/7/8) — email/password login then admin actions.
# Run from the project root: ./scripts/smoke.sh
set -euo pipefail

BACKEND="${BACKEND_URL:-http://127.0.0.1:8080}"
FRONTEND="${FRONTEND_URL:-http://127.0.0.1:3000}"

echo "==> /healthz on backend"
curl -fsS "$BACKEND/healthz" && echo

echo "==> Frontend serves index.html"
curl -fsS -o /dev/null -w "  HTTP %{http_code}\n" "$FRONTEND/"

# ----- student flow (no login) ---------------------------------------------
DEVICE_ID="dev-smoke-$(date +%s)-$RANDOM-uuid"
echo "==> Student device login (device_id=${DEVICE_ID:0:16}…)"
STUDENT_TOKEN=$(curl -fsS -X POST "$BACKEND/api/auth/device" \
  -H 'Content-Type: application/json' \
  -d "{\"device_id\":\"$DEVICE_ID\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
echo "  token=${STUDENT_TOKEN:0:8}…"

student=(-H "Authorization: Bearer $STUDENT_TOKEN")

echo "==> /api/auth/me as student"
curl -fsS "${student[@]}" "$BACKEND/api/auth/me" && echo

echo "==> Pick a free seat 2h from now"
START=$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=2)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
END=$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=3)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
echo "  window: $START .. $END"
SEATS=$(curl -fsS "${student[@]}" "$BACKEND/api/seats/available?start_at=$START&end_at=$END")
echo "  available: $SEATS"
SEAT_ID=$(echo "$SEATS" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])')

echo "==> Student creates booking on seat $SEAT_ID with display_name"
BOOKING=$(curl -fsS -X POST "${student[@]}" -H 'Content-Type: application/json' \
  -d "{\"seat_id\":$SEAT_ID,\"start_at\":\"$START\",\"end_at\":\"$END\",\"display_name\":\"Smoke Test\"}" \
  "$BACKEND/api/bookings")
echo "  $BOOKING"
BOOKING_ID=$(echo "$BOOKING" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

echo "==> Student lists own bookings"
curl -fsS "${student[@]}" "$BACKEND/api/bookings/me" && echo

echo "==> Student cancels booking $BOOKING_ID"
curl -fsS -X DELETE "${student[@]}" -o /dev/null -w "  HTTP %{http_code}\n" "$BACKEND/api/bookings/$BOOKING_ID"

# ----- admin flow ----------------------------------------------------------
echo "==> Admin login (seed credentials)"
ADMIN_TOKEN=$(curl -fsS -X POST "$BACKEND/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@mai.ru","password":"admin123"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
echo "  token=${ADMIN_TOKEN:0:8}…"

admin=(-H "Authorization: Bearer $ADMIN_TOKEN")

echo "==> Admin views all bookings (display_name should be present)"
curl -fsS "${admin[@]}" "$BACKEND/api/admin/bookings" && echo

echo "==> Admin reports for next 24h"
FROM=$(python3 -c 'import datetime; print(datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))')
TO=$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).strftime("%Y-%m-%dT%H:%M:%SZ"))')
curl -fsS "${admin[@]}" "$BACKEND/api/admin/reports?from=$FROM&to=$TO" && echo

echo "==> All good."
