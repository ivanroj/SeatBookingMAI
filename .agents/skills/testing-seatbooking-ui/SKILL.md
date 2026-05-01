# Testing the SeatBookingMAI role-gated UI

Use this when asked to verify the student/admin flows of `ivanroj/SeatBookingMAI` end-to-end through the browser.

## What the app actually does
- Single page at `http://localhost:3000` (frontend) backed by `http://localhost:8080` (Go backend).
- A landing screen «Кто вы?» role-gates the rest of the UI.
  - Student card → anonymous device session: a UUID is generated client-side, stored in `localStorage` as `deviceId`, and exchanged at `POST /api/auth/device` for a token. There is NO student login.
  - Admin card → email/password login at `POST /api/auth/login`. Seed account: `admin@mai.ru` / `admin123`.
- A student types their display name on every booking; it is stored in `bookings.display_name` and surfaced to admins as the «Студент» column in the All Bookings table.

## Bring up the stack
```bash
docker compose up -d --build
docker compose ps        # all three should be (healthy)
```
The healthchecks are real — wait until db/backend/frontend all report `(healthy)` before testing.

## Reset to a clean baseline before recording
The student device flow auto-creates hidden `role=user` rows. Reset bookings/sessions and any non-admin users so the test starts from a known state:
```bash
docker compose exec -T db psql -U postgres -d seat_booking -c \
  "TRUNCATE bookings, sessions RESTART IDENTITY CASCADE; DELETE FROM users WHERE role='user';"
```
DB has 3 seats seeded (A-01, A-02, B-01) and one admin user. Do NOT touch `seats` — IDs are referenced from the report assertion.

Also clear browser state at the start of the recording: open the page, then `localStorage.clear(); location.reload();` (or use a fresh incognito profile). Otherwise a leftover `studentToken` can auto-restore the student panel and silently defeat the T1 landing assertion.

## Three adversarial tests in one continuous recording
Goal: prove role-gating + device session + `display_name` round-trip in one shot. Run them in order so T3 can verify what T2 wrote.

### T1 — Landing actually role-gates the UI
Fresh load with empty `localStorage`. **Pass criteria:**
- Only the «Кто вы?» panel is rendered.
- No student name field, no admin email/password field visible.
- Status badge in the header reads exactly «Не выбрана роль».
- No «← Сменить роль» back button visible.

### T2 — Student flow without login
1. Click «Я студент / сотрудник» → student panel renders, header badge becomes «Устройство: <8-hex>…», log shows «Студенческая сессия по устройству …». No `POST /api/auth/login` is issued (this is the «без логина» proof).
2. Type a recognizable name like «Иван Студент» in «Имя».
3. Click «Показать свободные места» → exactly 3 rows (A-01, A-02, B-01).
4. Click «Забронировать» on A-01 → A-01 leaves Available, a new row appears in «Мои брони» with **Имя equal to what you typed**, Статус=`confirmed`.
5. Click «Отменить» → Статус flips to `canceled`, Cancel button disappears, A-01 returns to Available.

### T3 — Switching to admin only shows admin functions
1. Click «← Сменить роль» → landing.
2. Click «Я администратор» → enter `admin@mai.ru` / `admin123` → «Войти».
3. Header title becomes «Администратор», badge `admin@mai.ru (admin)`, only UC-5/6/7/8 sections render. No student widgets must remain visible.
4. Click «Загрузить все брони» → one row, **Студент = «Иван Студент»** (the name typed in T2), Статус=`canceled`. If you instead see something like `user#2`, the `display_name` pipeline is broken — flag it loudly.
5. Set the report range to bracket the booking time and click «Сформировать отчёт» → expect `total_bookings: 1`, `canceled_bookings: 1`, `by_seat: {"1": 1}`.

## Known gotcha — `[hidden]` vs `.panel { display: grid }`
The role-gating relies on `el.panel.hidden = true/false`. The native browser rule `[hidden] { display: none }` has the same specificity as `.panel { display: grid }`, and whichever comes later wins. The repo therefore needs an explicit `[hidden] { display: none !important; }` near the top of `frontend/styles.css`.

If during T1 you see all four panels (landing + student + admin login + admin) rendered at once, that override has been removed/regressed. Fix it before continuing — otherwise T1 and T3 are not actually testing role-gating.

## Reporting
- Single continuous recording for T1→T2→T3. Annotate `test_start` and one consolidated `assertion` per test.
- Post one PR comment with `<details>` sections, the most important one (T2) pre-expanded.
- Attach the full continuous recording mp4 to the user message.
- A separate `test-report.md` with inline screenshots is expected — do not send a text-only report.

## Devin Secrets Needed
None. The seed admin (`admin@mai.ru` / `admin123`) is set by the backend on first start; the student flow does not need any credentials.
