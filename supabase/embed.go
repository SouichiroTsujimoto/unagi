// Package supabase embeds the SQL migrations applied by the Supabase CLI
// (`supabase start`, `supabase db push`) and by GitHub integration's
// Deploy to production. Tests reuse the same files against a plain Postgres.
package supabase

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
