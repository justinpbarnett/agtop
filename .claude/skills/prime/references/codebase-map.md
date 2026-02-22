# Keep Codebase Reference

Church retention intelligence SaaS. Detects declining member engagement within 1-4 weeks. Pilot: New Life Church, Canton, GA. Live at keep.church.

## Data Flow

Planning Center sync → engagement records → inference engine (5 layers) → retention scoring → risk assessments → Monday Morning Report email

## Tech Stack

Next.js 15 / React 19, TypeScript, Tailwind CSS 4, Drizzle ORM, PostgreSQL (Supabase), Supabase Auth (church_id in JWT app_metadata), Resend emails, Vercel deployment. Task runner: `justfile`. Package manager: pnpm.

## Commands

| Command | Action |
|---------|--------|
| `just start` | Dev server + background services |
| `just check` | lint + typecheck + test + e2e |
| `just test` | Vitest unit tests |
| `just test-e2e` | Playwright E2E tests |
| `just lint` | ESLint |
| `just typecheck` | tsc --noEmit |
| `just db-upgrade` | drizzle-kit push |
| `just adw "prompt"` | AI Developer Workflow |

## Database Schema (10 tables, all RLS-enabled)

All in `src/db/schema/`. Drizzle ORM client: `src/db/index.ts`.

| Table | Key Columns | Notes |
|-------|-------------|-------|
| `churches` | id, name, timezone, settings (jsonb), onboardedAt | Multi-tenant root. `settings` stores per-church engine config, report config, plan tier |
| `households` | id, churchId, name, address, joinedDate, externalIds | Primary unit of retention analysis |
| `people` | id, churchId, householdId, firstName, lastName, email, phone, dateOfBirth, roleInHousehold, gender | Roles: head, spouse, child. Indexed on church, household, email, phone, name |
| `engagements` | id, churchId, personId, householdId, type, date, source, confidence, externalId, metadata | Types: sunday_attendance, kids_checkin, giving, volunteering, group_attendance, aggregate_headcount, absence. Deduped by externalId |
| `risk_assessments` | id, churchId, householdId, weekDate, riskLevel, reason, recommendedAction, triggeringPersonId, baselineAttendance, recentAttendance, attendanceVelocity, consecutiveMisses, givingActive, volunteeringActive, groupActive, confidenceLevel | Unique on (churchId, householdId, weekDate) |
| `groups` | id, churchId, name, type, leaderId, meetingDay, isActive | Small groups, teams, classes |
| `group_memberships` | id, churchId, groupId, personId, role, joinedAt, leftAt | Unique on (groupId, personId) |
| `users` | id, churchId, role, personId, email, name | Roles: keep_admin, church_admin. PK is Supabase auth UUID |
| `integrations` | id, churchId, provider, status, credentialsEncrypted (bytea), lastSyncAt, syncCursor, errorMessage | AES-256-GCM encrypted credentials |
| `leads` | id, email, churchName, currentAttendance, previousAttendance, guestsPerMonth, peopleLost, revenueImpact, retentionRate, source, status | From backdoor calculator lead magnet |

## Inference Engine (src/lib/engine/inference.ts)

5-layer attendance reconstruction. Pure function: engagements in, per-person-per-Sunday AttendanceMap out. All layers respect an exclusion set built from `absence` engagements. Configurable per-church via `churches.settings.inferenceConfig` (deep-merged with defaults in `inference-config.ts`).

| Layer | Name | Logic | Confidence |
|-------|------|-------|------------|
| 1 | Raw Observed | Direct `sunday_attendance` records on Sundays | 1.0 |
| 2 | Direct Evidence | Signal types on Sunday dates: kids_checkin (1.0), volunteering (1.0), giving (0.9), group_attendance (0.9). Giving has 3-day attribution window to nearest Sunday | per signal |
| 3 | Household Linkage | Child (<=11) checked in → infer head/spouse adults. Adult present → infer cohabiting children (<13) and head/spouse. Full-family signal → infer teens (<=17) | 0.85 (teens: 0.75) |
| 4 | Temporal Pattern Fill | For people with >=3 confirmed Sundays and >40% observation rate, fill single-week gaps between confirmed dates. Optional streak-based edge fill | 0.7 |
| 5 | Headcount Reconciliation | If inferred < aggregate headcount, fill with highest-probability candidates (recency-weighted observation rate). Tracks visitor estimates | 0.6 |

## Retention Scoring (src/lib/engine/retention.ts)

Per-person risk computed then rolled up to household. Windows: recent=4 weeks, baseline=12 weeks, minimum=8 weeks. Adults only (age >= 13). Configurable per-church via `churches.settings.retentionConfig` (deep-merged with defaults in `retention-config.ts`).

**Risk classification (priority order):**
1. **Inactive** — baseline rate < 10%
2. **Red** — 3+ consecutive misses AND signal dropped (giving/volunteering/group), OR 6+ absolute consecutive misses
3. **Orange** — 3+ consecutive misses, OR velocity drop > -30% (baseline > 30%), OR relative velocity drop > -50% (baseline > 15%)
4. **Yellow** — velocity drop > -15% (baseline > 20%), OR relative velocity drop > -25% (baseline > 15%), OR giving/group dropped while attendance ok
5. **Green** — engagement stable

**Household rollup:** worst adult member's risk level wins. Ties broken by signal-drop count, then consecutive misses.

**Escalation:** orange → red after 4 consecutive weeks; yellow → orange after 6 consecutive weeks.

**Signal health:** giving/volunteering/group each tracked as active (recent > 0) and dropped (recent/baseline ratio < 0.3 when baseline > threshold).

## Pipeline (src/lib/engine/pipeline.ts)

Orchestrator: loads church → computes most recent Sunday in church timezone → generates 52-week Sunday window → loads engagements + people → runs inference → runs retention scoring → applies escalation from prior assessments → batch upserts risk_assessments (100 per batch).

## Planning Center Integration (src/lib/planning-center/)

- `client.ts` — PAT-based auth (Basic base64), auto-pagination, rate limit handling (100 req/20s threshold), retry with exponential backoff
- `sync.ts` — Orchestrator: people → groups → check-ins → giving → services. Full or incremental (since lastSyncAt)
- `sync-people.ts`, `sync-checkins.ts`, `sync-giving.ts`, `sync-groups.ts`, `sync-services.ts` — Individual adapters
- `mapper.ts` — PC data model → Keep engagement log format
- `types.ts` — PC JSON:API types (PCPerson, PCCheckIn, PCDonation, PCGroup, etc.)

## API Routes

| Route | Purpose |
|-------|---------|
| `src/app/api/health/route.ts` | Health check |
| `src/app/api/cron/sync/route.ts` | Daily PC sync for all connected integrations |
| `src/app/api/cron/inference/route.ts` | Monday inference pipeline for all churches |
| `src/app/api/cron/report/route.ts` | Hourly on Mondays, sends Monday Morning Report at 7 AM local per church timezone |

## Auth (src/lib/auth/)

- `get-church-id.ts` — Reads church_id from Supabase JWT app_metadata
- `roles.ts` — Roles: keep_admin, church_admin
- `require-admin.ts` — Admin guard
- `verify-cron-secret.ts` — CRON_SECRET auth for cron routes
- `src/middleware.ts` — Next.js middleware, updateSession for all routes except static/health/cron

## App Structure (src/app/)

| Path | Purpose |
|------|---------|
| `layout.tsx` | Root layout (fonts: Instrument Serif, DM Sans, JetBrains Mono) |
| `page.tsx` | Landing page |
| `login/page.tsx` | Login page |
| `invite/page.tsx` | Invite acceptance |
| `backdoor/page.tsx` | Free retention calculator (lead magnet) |
| `(dashboard)/layout.tsx` | Dashboard layout with Sidebar + auth guard |
| `(dashboard)/dashboard/page.tsx` | Risk distribution stat cards, priority households, data coverage |
| `(dashboard)/households/page.tsx` | Household browser with search, sort, filter by risk level |
| `(dashboard)/households/[householdId]/page.tsx` | Household detail (engagement timeline, risk trend, family members) |
| `(dashboard)/concerns/[householdId]/page.tsx` | Redirects to /households/[id] |
| `(dashboard)/admin/seed/page.tsx` | Generate test data (small/medium/large presets) |
| `(dashboard)/admin/inference/page.tsx` | Run inference, view results, report config |
| `(dashboard)/admin/integrations/page.tsx` | Planning Center connect/sync/disconnect |
| `(dashboard)/admin/users/page.tsx` | User management, keep_admin panel |
| `(dashboard)/admin/emails/page.tsx` | Email preview |
| `(dashboard)/admin/leads/page.tsx` | Lead management |

## Key Components (src/components/)

sidebar.tsx, stat-card.tsx, risk-badge.tsx, risk-distribution-bar.tsx, risk-trend.tsx, priority-list.tsx, household-card.tsx, engagement-timeline.tsx, signal-checklist.tsx, data-coverage.tsx, search-input.tsx, pagination-controls.tsx, landing/*.tsx

## Seed Data Generator (src/lib/seed/)

Deterministic test data generator with PRNG. Presets: small (100 HH), medium (300 HH), large (600 HH). Features: attendance category drift via Markov chain, seasonal modulation, demo narratives for hand-crafted risk scenarios, observability filter.

## Email (src/lib/email/)

Monday Morning Report (monday-report.tsx), invite email, reset password email, application confirmation, backdoor report. Sent via Resend.

## Testing

- **Unit tests:** Vitest, `src/__tests__/` (engine, auth, API, components, planning-center, seed, email)
- **E2E tests:** Playwright, `e2e/` with 3 projects: chromium (regular user), keep-admin, mobile
- **Test factories:** `src/test/factories.ts`

## Specs & ADWs

- Specs live in `specs/` — implementation plans created before coding
- ADW orchestration in `adws/` — Python scripts using Claude Code agents
- Skills in `.claude/skills/` — slash command definitions
