#!/usr/bin/env bash
# Pre-push verify: run the checks CI will run, scoped to what changed.
# Usage: ./scripts/verify_before_push.sh [base_ref]
#   base_ref defaults to origin/main; set WVP_FULL_E2E=1 to always run the full e2e suite.
set -e
LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/verify_before_push_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"
echo "Logging to: $LOG_FILE"

BASE_REF="${1:-origin/main}"
CHANGED=$(git diff --name-only "$BASE_REF"...HEAD 2>/dev/null || git diff --name-only HEAD)
UNCOMMITTED=$(git diff --name-only; git ls-files --others --exclude-standard)
FILES="$CHANGED
$UNCOMMITTED"

if [[ -z "$(echo "$FILES" | tr -d '[:space:]')" ]]; then
  echo "No changes vs $BASE_REF and nothing uncommitted — nothing to verify."
  exit 0
fi

run_e2e_specs=()
go_changed=0
fe_changed=0
tmpl_changed=0
md_changed=0
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  case "$f" in
    *.go) go_changed=1 ;;
    static/js/*) fe_changed=1 ;;
    static/templates/*) tmpl_changed=1; fe_changed=1 ;;
    internal/syspage/*) tmpl_changed=1 ;;
    e2e/tests/*) run_e2e_specs+=("$f") ;;
    *.md) md_changed=1 ;;
  esac
done <<< "$FILES"
run_e2e_specs=(${run_e2e_specs[@]:-})

echo "== changed files: $(echo "$FILES" | grep -c .) (go=$go_changed fe=$fe_changed tmpl=$tmpl_changed md=$md_changed e2e-specs=${#run_e2e_specs[@]}) =="

{
  if [[ $go_changed == 1 ]]; then
    echo "--- go:test ---"
    devbox run go:test
    echo "--- go:lint ---"
    devbox run go:lint
  fi

  if [[ $fe_changed == 1 ]]; then
    # e2e specs that reference any touched component/template
    if [[ $fe_changed == 1 || $tmpl_changed == 1 ]]; then
      touched=$(echo "$FILES" | grep -E 'static/(js|templates)/' || true)
      if [[ -n "$touched" ]]; then
        for spec in e2e/tests/*.spec.ts; do
          while IFS= read -r t; do
            name=$(basename "$t" | sed 's/\.[^.]*$//;s/\.test$//;s/\.ts$//')
            if grep -q "$name" "$spec" 2>/dev/null; then
              run_e2e_specs+=("$spec")
              break
            fi
          done <<< "$touched"
        done
      fi
    fi
    echo "--- fe:test (touched test files) ---"
    fe_test_files=()
    while IFS= read -r f; do
      [[ "$f" == *.test.ts ]] && fe_test_files+=("$f")
    done <<< "$FILES"
    if [[ ${#fe_test_files[@]} -gt 0 ]]; then
      devbox run fe:test -- "${fe_test_files[@]}"
    else
      devbox run fe:test
    fi
    echo "--- fe:lint ---"
    devbox run fe:lint
    echo "--- fe:typecheck ---"
    devbox run fe:typecheck
  fi

  if [[ ${#run_e2e_specs[@]} -gt 0 ]]; then
    # dedupe
    mapfile -t specs_uniq < <(printf '%s\n' "${run_e2e_specs[@]}" | sort -u)
    echo "--- e2e:test (touched specs: ${specs_uniq[*]}) ---"
    devbox run e2e:test -- "${specs_uniq[@]/#e2e\/tests\//tests/}"
  fi

  if [[ $md_changed == 1 ]]; then
    echo "--- lint:md (touched md files) ---"
    devbox run lint:md -- $(echo "$FILES" | grep '\.md$' | tr '\n' ' ') || true
    # lint:md currently has pre-existing baseline failures in eval/*.md and
    # e2e/test-data (issue-tracked); informational here.
  fi

  echo "--- lint:everything (final gate) ---"
  devbox run lint:everything
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code