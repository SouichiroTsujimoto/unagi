default: build

init:
    go run ./cmd/init

generate:
    go tool templ generate -path internal/web

css:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -x ./tools/tailwindcss ]]; then
      echo "tools/tailwindcss not found; run: just init" >&2
      exit 1
    fi
    ./tools/tailwindcss --silent -i ./internal/web/css/styles.css -o ./static/app.css --minify

# Regenerate syntax-highlight CSS (chroma) for article code blocks.
chroma-css:
    go run ./tools/gen-chroma-css .

# Fetch pinned ESM deps into static/vendor (Node not required). Versions: static/vendor/VERSIONS.txt
vendor-islands:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p static/vendor
    ISLAND_VER=5.0.1
    PREACT_VER=10.26.4
    HTM_VER=3.1.1
    PCE_VER=4.3.0
    curl -fsSL "https://unpkg.com/@11ty/is-land@${ISLAND_VER}/is-land.js" -o static/vendor/is-land.js
    curl -fsSL "https://unpkg.com/preact@${PREACT_VER}/dist/preact.module.js" -o static/vendor/preact.module.js
    curl -fsSL "https://unpkg.com/preact@${PREACT_VER}/hooks/dist/hooks.module.js" -o static/vendor/preact-hooks.module.js
    curl -fsSL "https://unpkg.com/htm@${HTM_VER}/dist/htm.module.js" -o static/vendor/htm.module.js
    curl -fsSL "https://unpkg.com/htm@${HTM_VER}/preact/index.mjs" -o static/vendor/htm-preact.module.js
    curl -fsSL "https://unpkg.com/preact-custom-element@${PCE_VER}/dist/preact-custom-element.esm.js" -o static/vendor/preact-custom-element.esm.js
    printf '%s\n' \
      '# Vendored island runtime (pinned). Re-fetch with: just vendor-islands' \
      "is-land=@11ty/is-land@${ISLAND_VER}" \
      "preact=preact@${PREACT_VER}" \
      "htm=htm@${HTM_VER}" \
      "preact-custom-element=preact-custom-element@${PCE_VER}" \
      > static/vendor/VERSIONS.txt
    echo "vendored island deps into static/vendor"

css-watch:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -x ./tools/tailwindcss ]]; then
      echo "tools/tailwindcss not found; run: just init" >&2
      exit 1
    fi
    # --watch=always keeps the process alive when stdin is closed (e.g. under just run).
    ./tools/tailwindcss --silent -i ./internal/web/css/styles.css -o ./static/app.css --watch=always

fmt:
    go tool templ fmt .
    gofmt -w ./cmd ./internal ./static

tidy: generate
    go mod tidy

test: generate
    # Schema: supabase/migrations (local: `just supabase-start`, CI: applied in OpenTest)
    go test ./...

vet: generate
    go vet ./...

build: generate css
    CGO_ENABLED=0 go build -o bin/server ./cmd/server

# Local Supabase (Postgres + Auth + Storage). Requires Docker + supabase CLI.
supabase-start:
    supabase start

supabase-stop:
    supabase stop

supabase-status:
    supabase status -o env

# Linux amd64 binary (Vercel container / distroless).
build-linux:
    #!/usr/bin/env bash
    set -euo pipefail
    version="$(git describe --tags --always --dirty 2>/dev/null || date -u +%Y%m%d%H%M%S)"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=${version}" -o bin/server-linux-amd64 ./cmd/server
    echo "wrote bin/server-linux-amd64 (${version})"

# Generate custom listen-banner ASCII from [banner].image (ascii-image-converter).
# No-op when image is unset. Writes sibling <name>-ascii.txt (+ .sha256); commit both.
logo:
    go run ./cmd/logo

# Dev server. Examples: `just run` / `just run logo=false` / `just run tui=false`
# Defaults come from .unigo.toml (then UNIGO_* env). CLI tokens override.
# just passes `key=value` as positional strings, so we parse tokens ourselves.
run args="" *more:
    #!/usr/bin/env bash
    set -euo pipefail
    parse_bool() {
      local name="$1" value="$2"
      case "$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')" in
        true|yes|on|1) printf 'true' ;;
        false|no|off|0) printf 'false' ;;
        *)
          echo "invalid ${name}=${value} (use true or false)" >&2
          exit 1
          ;;
      esac
    }
    logo=""
    tui=""
    pos=0
    apply_token() {
      local tok="$1"
      [[ -z "$tok" ]] && return 0
      case "$tok" in
        logo=*)
          logo="$(parse_bool logo "${tok#logo=}")"
          ;;
        tui=*)
          tui="$(parse_bool tui "${tok#tui=}")"
          ;;
        true|false|yes|no|on|off|1|0|TRUE|FALSE|Yes|No|On|Off)
          case "$pos" in
            0) logo="$(parse_bool logo "$tok")" ;;
            1) tui="$(parse_bool tui "$tok")" ;;
            *)
              echo "unexpected argument: ${tok}" >&2
              exit 1
              ;;
          esac
          pos=$((pos + 1))
          ;;
        *)
          echo "invalid argument: ${tok} (use logo=true|false and/or tui=true|false)" >&2
          exit 1
          ;;
      esac
    }
    apply_token "{{args}}"
    if [[ -n "{{more}}" ]]; then
      for tok in {{more}}; do
        apply_token "$tok"
      done
    fi
    cmd=(go run ./cmd/dev)
    if [[ -n "$logo" ]]; then
      cmd+=(-logo="$logo")
    fi
    if [[ -n "$tui" ]]; then
      cmd+=(-tui="$tui")
    fi
    "${cmd[@]}"

check: generate css
    go test ./...
    go vet ./...
    CGO_ENABLED=0 go build -o bin/server ./cmd/server

clean:
    rm -rf bin tmp
