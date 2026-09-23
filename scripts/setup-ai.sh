#!/usr/bin/env bash
set -o pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/setup-ai.sh [--agent cursor|claude|codex|opencode]... [--install] [--setup-labels]

Default mode checks repository prerequisites without changing files.
--agent selects an agent explicitly; repeat it to check multiple agents.
--install invokes documented skill installers where available and prints steps
          requiring an agent UI. Installation is interactive and opt-in.
--setup-labels creates missing default GitHub labels only; it never changes
               existing labels. Requires a GitHub remote and authenticated gh.

Exit status: 0 READY, 2 NEEDS CONFIRMATION, 1 BLOCKED.
USAGE
}

agents=()
install_mode=false
setup_labels=false
while (($#)); do
  case "$1" in
    --agent)
      if (($# < 2)); then printf 'Missing value for --agent\n' >&2; usage >&2; exit 2; fi
      case "$2" in
        cursor|claude|codex|opencode) agents+=("$2") ;;
        *) printf 'Unsupported agent: %s\n' "$2" >&2; usage >&2; exit 2 ;;
      esac
      shift 2
      ;;
    --install) install_mode=true; shift ;;
    --setup-labels) setup_labels=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown argument: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
done

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
errors=0
warnings=0
manual_checks=0
ok() { printf 'OK   %s\n' "$1"; }
warn() { printf 'WARN %s\n' "$1"; warnings=$((warnings + 1)); }
fail() { printf 'FAIL %s\n' "$1"; errors=$((errors + 1)); }
manual() { printf 'CHECK %s\n' "$1"; manual_checks=$((manual_checks + 1)); }

# If no explicit agent was selected, detect available CLIs as candidates.
if ((${#agents[@]} == 0)); then
  command -v cursor >/dev/null 2>&1 && agents+=(cursor)
  command -v claude >/dev/null 2>&1 && agents+=(claude)
  command -v codex >/dev/null 2>&1 && agents+=(codex)
  command -v opencode >/dev/null 2>&1 && agents+=(opencode)
fi
# Remove duplicates while retaining order.
unique_agents=()
for agent in "${agents[@]}"; do
  seen=false
  for existing in "${unique_agents[@]}"; do [[ "$agent" == "$existing" ]] && seen=true; done
  [[ "$seen" == true ]] || unique_agents+=("$agent")
done
agents=("${unique_agents[@]}")

printf 'AI workflow readiness for %s\n\n' "$root"

if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  ok 'Git repository detected'
else
  fail 'Not inside a Git repository'
fi

remote="$(git -C "$root" remote get-url origin 2>/dev/null || true)"
repo=""
if [[ -n "$remote" ]]; then
  ok "origin remote configured: $remote"
  normalized_remote="${remote%.git}"
  if [[ "$normalized_remote" =~ github\.com[:/]([^/]+/[^/]+)$ ]]; then repo="${BASH_REMATCH[1]}"; fi
else
  fail 'No origin remote configured; GitHub issue and label checks are unavailable'
fi

if command -v gh >/dev/null 2>&1; then
  gh_version="$(gh --version 2>/dev/null | awk 'NR == 1 { print; exit }')"
  [[ -n "$gh_version" ]] || gh_version='gh installed'
  ok "GitHub CLI found: $gh_version"
  if gh auth status >/dev/null 2>&1; then
    ok 'GitHub CLI authenticated'
  else
    fail 'GitHub CLI is not authenticated; run gh auth login before GitHub operations'
  fi
else
  fail 'GitHub CLI (gh) is not installed; install it to create or update GitHub Issues'
fi

required_files=(
  AGENTS.md
  docs/engineering/development-workflow.md
  docs/agents/issue-tracker.md docs/agents/triage-labels.md docs/agents/domain.md
)
for file in "${required_files[@]}"; do
  if [[ -f "$root/$file" ]]; then ok "Workflow file present: $file"; else fail "Workflow file missing: $file"; fi
done

if ((${#agents[@]})); then
  printf 'Selected/detected agent candidates: %s\n' "${agents[*]}"
  for agent in "${agents[@]}"; do
    case "$agent" in
      claude) local_base="$root/.claude/skills";;
      cursor) local_base="$root/.cursor/skills";;
      codex) local_base="$root/.codex/skills";;
      opencode) local_base="$root/.opencode/skills";;
    esac
    shared_base="$root/.agents/skills"
    matt_local=false
    superpowers_local=false
    matt_shared=false
    superpowers_shared=false
    if [[ -f "$local_base/setup-matt-pocock-skills/SKILL.md" ]]; then matt_local=true; fi
    if [[ -f "$local_base/superpowers/SKILL.md" || -f "$local_base/using-superpowers/SKILL.md" ]]; then superpowers_local=true; fi
    for base in "$shared_base" "$HOME/.agents/skills"; do
      [[ -f "$base/setup-matt-pocock-skills/SKILL.md" ]] && matt_shared=true
      [[ -f "$base/superpowers/SKILL.md" || -f "$base/using-superpowers/SKILL.md" ]] && superpowers_shared=true
    done
    if [[ "$agent" == claude ]]; then
      manual 'Verify Claude Code loaded the repository instructions and has both Matt Pocock Skills and Superpowers enabled; plugin state is not reliably visible from this generic script.'
    else
      if [[ "$matt_local" == true ]]; then
        ok "Matt Pocock Skills files found in the $agent-specific skills directory"
      elif [[ "$matt_shared" == true ]]; then
        manual "Matt Pocock Skills exist only in a shared skills directory; confirm $agent loads them."
      else
        manual "Install and verify Matt Pocock Skills for $agent."
      fi
      if [[ "$superpowers_local" == true ]]; then
        ok "Superpowers files found in the $agent-specific skills directory"
      elif [[ "$superpowers_shared" == true ]]; then
        manual "Superpowers exists only in a shared skills directory; confirm $agent loads it."
      else
        manual "Install and verify Superpowers for $agent."
      fi
    fi
  done
else
  manual 'No agent selected/detected; rerun with --agent cursor, --agent claude, --agent codex, or --agent opencode'
fi

labels=()
add_required_label() {
  local candidate="$1"
  [[ -z "$candidate" ]] && return
  for existing in "${labels[@]}"; do [[ "$existing" == "$candidate" ]] && return; done
  labels+=("$candidate")
}

# Read configured triage labels rather than assuming every adopted project uses
# the defaults. Also include labels declared by the repository's issue templates.
while IFS= read -r label; do add_required_label "$label"; done < <(
  awk -F'|' '/^\| `/ { label=$3; gsub(/`/, "", label); gsub(/^[[:space:]]+|[[:space:]]+$/, "", label); if (label != "") print label }' \
    "$root/docs/agents/triage-labels.md"
)
for issue_template in "$root"/.github/ISSUE_TEMPLATE/*.md; do
  [[ -f "$issue_template" ]] || continue
  while IFS= read -r label; do add_required_label "$label"; done < <(
    awk '
      /^labels:[[:space:]]*\[/ {
        line=$0
        sub(/^[^[]*\[/, "", line)
        sub(/\].*$/, "", line)
        gsub(/["\047]/, "", line)
        count=split(line, values, ",")
        for (i=1; i<=count; i++) {
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", values[i])
          if (values[i] != "") print values[i]
        }
      }
    ' "$issue_template"
  )
done

if [[ -n "$remote" ]] && command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  if [[ -n "$repo" ]]; then
    if ! existing_labels="$(gh -R "$repo" label list --limit 1000 --json name --jq '.[].name' 2>/dev/null)"; then
      fail "Could not read labels from GitHub repository $repo"
      existing_labels=""
    else
      missing_labels=()
      for label in "${labels[@]}"; do
        if printf '%s\n' "$existing_labels" | awk -v name="$label" '$0 == name { found=1 } END { exit !found }'; then
          ok "GitHub label exists: $label"
        else
          missing_labels+=("$label")
        fi
      done
      if ((${#missing_labels[@]})); then
        if [[ "$setup_labels" == true ]]; then
          for label in "${missing_labels[@]}"; do
            color=ededed
            description='Configured repository issue label'
            case "$label" in
              bug) color=d73a4a; description='Bug report' ;;
              enhancement) color=a2eeef; description='Feature or enhancement' ;;
              needs-triage) color=fbca04; description='Maintainer triage required' ;;
              needs-info) color=d876e3; description='Waiting for reporter information' ;;
              ready-for-agent) color=0e8a16; description='Requirements ready for agent work' ;;
              ready-for-human) color=bfdadc; description='Requires human implementation or judgment' ;;
              wontfix) color=ffffff; description='Request will not be actioned' ;;
            esac
            if gh -R "$repo" label create "$label" --color "$color" --description "$description" >/dev/null; then
              ok "Created missing GitHub label: $label"
            else
              fail "Could not create GitHub label: $label"
            fi
          done
        else
          for label in "${missing_labels[@]}"; do fail "Required GitHub label missing: $label (run --setup-labels to create missing defaults)"; done
        fi
      fi
    fi
  else
    if [[ "$setup_labels" == true ]]; then
      fail 'Could not parse a github.com repository from origin; refusing to create labels without a verified target repository'
    else
      warn 'Could not parse a github.com repository from origin; label check skipped (GitHub Enterprise remotes may need manual verification)'
    fi
  fi
elif [[ "$setup_labels" == true ]]; then
  fail '--setup-labels requires a GitHub origin, gh, and authenticated GitHub CLI'
else
  fail 'GitHub labels not checked; connect a remote and authenticate gh to verify issue-template labels'
fi

if [[ "$install_mode" == true ]]; then
  non_claude_agent=false
  for agent in "${agents[@]}"; do [[ "$agent" != claude ]] && non_claude_agent=true; done
  if [[ "$non_claude_agent" == true ]]; then
    if command -v npx >/dev/null 2>&1; then
      printf '\nRunning the official Matt Pocock Skills installer for the selected non-Claude agent(s). Select the target agents and include setup-matt-pocock-skills.\n'
      npx skills@latest add mattpocock/skills || fail 'Matt Pocock Skills installer did not complete'
    else
      fail 'npx is required to install Matt Pocock Skills for Cursor, Codex, or OpenCode'
    fi
  fi
  for agent in "${agents[@]}"; do
    case "$agent" in
      claude)
        if command -v claude >/dev/null 2>&1; then
          printf '\nInstalling Matt Pocock Skills for Claude Code using its official plugin installer.\n'
          claude plugins install mattpocock-skills || fail 'Matt Pocock Skills plugin installation failed for Claude Code'
        else
          manual 'Install Matt Pocock Skills for Claude Code with: /plugin install mattpocock-skills'
        fi
        printf 'Install Superpowers for Claude Code with: /plugin install superpowers@claude-plugins-official\n'
        ;;
      cursor)
        printf 'Install Superpowers in Cursor Agent chat with: /add-plugin superpowers\n'
        ;;
      codex)
        printf 'Install Superpowers in Codex with /plugins, search Superpowers, then select Install Plugin.\n'
        ;;
      opencode)
        printf 'Install Superpowers for OpenCode using https://raw.githubusercontent.com/obra/superpowers/refs/heads/main/.opencode/INSTALL.md\n'
        ;;
    esac
  done
  if ((${#agents[@]} == 0)); then warn 'Select an agent with --agent before using --install'; fi
  if ((${#agents[@]})); then
    printf '\nRechecking local readiness after supported install steps:\n'
    post_install_args=()
    for agent in "${agents[@]}"; do post_install_args+=(--agent "$agent"); done
    "$root/scripts/setup-ai.sh" "${post_install_args[@]}"
  fi
fi

printf '\nSummary: %d blocking issue(s), %d warning(s), %d agent-side confirmation step(s).\n' "$errors" "$warnings" "$manual_checks"
if ((errors > 0)); then
  printf 'Status: BLOCKED (exit 1) — resolve the blocking issues before relying on GitHub workflow operations.\n'
  exit 1
elif ((warnings > 0 || manual_checks > 0)); then
  printf 'Status: NEEDS CONFIRMATION (exit 2) — complete the listed agent-side checks before relying on the workflow.\n'
  exit 2
else
  printf 'Status: READY (exit 0) — all checked prerequisites passed.\n'
  exit 0
fi
