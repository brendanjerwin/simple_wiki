#!/usr/bin/env bash

# Query SonarCloud project data from the CLI.
# Usage:
#   ./scripts/run_sonar_query.sh issues [--severity BLOCKER|CRITICAL|MAJOR|MINOR]
#   ./scripts/run_sonar_query.sh gate
#   ./scripts/run_sonar_query.sh coverage
#   ./scripts/run_sonar_query.sh hotspots

set -e

# Load token from .env if not already set
if [[ -z "$SONAR_TOKEN" && -f .env ]]; then
  # shellcheck disable=SC2046
  export $(grep -v '^#' .env | xargs)
fi

if [[ -z "$SONAR_TOKEN" ]]; then
  echo "Error: SONAR_TOKEN not set. Create a .env file with your token or export it." >&2
  echo "Generate one at https://sonarcloud.io/account/security" >&2
  exit 1
fi

BASE_URL="https://sonarcloud.io/api"
PROJECT="brendanjerwin_simple_wiki"

sonar_api() {
  local endpoint="$1"
  curl -s -H "Authorization: Bearer $SONAR_TOKEN" "$BASE_URL/$endpoint"
  return 0
}

case "${1:-issues}" in
  issues)
    SEVERITY="${2:+&severities=$2}"
    # Strip the leading -- from severity flags like --severity
    SEVERITY="${SEVERITY/--severity/}"

    PARAMS="componentKeys=$PROJECT&statuses=OPEN,CONFIRMED&ps=50${SEVERITY}"
    RESPONSE=$(sonar_api "issues/search?$PARAMS")

    TOTAL=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total', 0))")
    echo "Open issues: $TOTAL"
    echo ""

    echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for issue in data.get('issues', []):
    sev = issue.get('severity', '?')
    rule = issue.get('rule', '?')
    comp = issue.get('component', '?').replace('brendanjerwin_simple_wiki:', '')
    line = issue.get('line', '?')
    msg = issue.get('message', '')[:120]
    print(f'  [{sev}] {comp}:{line}')
    print(f'    {rule}: {msg}')
    print()
"
    ;;

  gate)
    RESPONSE=$(sonar_api "qualitygates/project_status?projectKey=$PROJECT")
    echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
status = data.get('projectStatus', {})
print(f\"Quality Gate: {status.get('status', '?')}\")
print()
for cond in status.get('conditions', []):
    metric = cond.get('metricKey', '?')
    actual = cond.get('actualValue', '?')
    threshold = cond.get('errorThreshold', '?')
    st = cond.get('status', '?')
    symbol = '✓' if st == 'OK' else '✗'
    print(f'  {symbol} {metric}: {actual} (threshold: {threshold})')
"
    ;;

  coverage)
    METRICS="coverage,new_coverage,lines_to_cover,uncovered_lines,line_coverage"
    RESPONSE=$(sonar_api "measures/component?component=$PROJECT&metricKeys=$METRICS")
    echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
comp = data.get('component', {})
print(f\"Project: {comp.get('name', '?')}\")
print()
for m in comp.get('measures', []):
    metric = m.get('metric', '?')
    value = m.get('value', '?')
    print(f'  {metric}: {value}')
"
    ;;

  hotspots)
    RESPONSE=$(sonar_api "hotspots/search?projectKey=$PROJECT&status=TO_REVIEW&ps=50")
    echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
total = data.get('paging', {}).get('total', 0)
print(f'Security hotspots to review: {total}')
print()
for h in data.get('hotspots', []):
    comp = h.get('component', '?').replace('brendanjerwin_simple_wiki:', '')
    line = h.get('line', '?')
    msg = h.get('message', '')[:120]
    vuln = h.get('vulnerabilityProbability', '?')
    print(f'  [{vuln}] {comp}:{line}')
    print(f'    {msg}')
    print()
"
    ;;

  *)
    echo "Usage: $0 {issues|gate|coverage|hotspots}"
    echo ""
    echo "Commands:"
    echo "  issues [SEVERITY]  - List open issues (BLOCKER, CRITICAL, MAJOR, MINOR)"
    echo "  gate               - Show quality gate status"
    echo "  coverage           - Show coverage metrics"
    echo "  hotspots           - List security hotspots to review"
    exit 1
    ;;
esac
