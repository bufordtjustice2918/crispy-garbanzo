#!/bin/bash
# Clawgress appliance shell integration.
# Intercepts VyOS-style commands (show, configure, set, commit, etc.)
# and routes them to clawgressctl. Everything else passes to bash.

export PATH="/usr/local/sbin:$PATH"
export CLAWGRESS_ADMIN_URL="http://127.0.0.1:8080"

# --- Colors ---
_C_RESET='\033[0m'
_C_BOLD='\033[1m'
_C_DIM='\033[2m'
_C_GREEN='\033[0;32m'
_C_RED='\033[0;31m'
_C_YELLOW='\033[0;33m'
_C_CYAN='\033[0;36m'
_C_BLUE='\033[0;34m'
_C_BGREEN='\033[1;32m'
_C_BRED='\033[1;31m'
_C_BCYAN='\033[1;36m'

# --- Spinner animation ---
_clawgress_spinner() {
    local msg="$1"
    local pid="$2"
    local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
    local i=0
    while kill -0 "$pid" 2>/dev/null; do
        printf "\r  ${_C_CYAN}${frames[$((i % ${#frames[@]}))]}${_C_RESET} ${msg}" >&2
        sleep 0.1
        i=$((i + 1))
    done
    printf "\r" >&2
}

# --- Status badge ---
_clawgress_badge() {
    local status="$1" text="$2"
    case "$status" in
        ok|pass|active|allow)  printf "${_C_BGREEN}[%s]${_C_RESET}" "$text" ;;
        fail|deny|disabled)    printf "${_C_BRED}[%s]${_C_RESET}" "$text" ;;
        warn|alert_only)       printf "${_C_YELLOW}[%s]${_C_RESET}" "$text" ;;
        *)                     printf "${_C_DIM}[%s]${_C_RESET}" "$text" ;;
    esac
}

# --- Appliance command aliases ---

show() {
    case "$1" in
        agents)
            echo -e "${_C_BCYAN}Agents${_C_RESET}"
            echo -e "${_C_DIM}$(printf '%.0s─' {1..60})${_C_RESET}"
            curl -sf "$CLAWGRESS_ADMIN_URL/v1/agents" | jq -r '.[] | "\(.agent_id)\t\(.team_id)\t\(.status)"' | while IFS=$'\t' read -r id team status; do
                local badge=$(_clawgress_badge "$status" "$status")
                printf "  %-20s %-15s %b\n" "$id" "$team" "$badge"
            done
            ;;
        policies)
            echo -e "${_C_BCYAN}Policies${_C_RESET}"
            echo -e "${_C_DIM}$(printf '%.0s─' {1..60})${_C_RESET}"
            curl -sf "$CLAWGRESS_ADMIN_URL/v1/policies" | jq -r '.[] | "\(.policy_id)\t\(.agent_id)\t\(.action)\t\(.domains | join(","))"' | while IFS=$'\t' read -r id agent action domains; do
                local badge=$(_clawgress_badge "$action" "$action")
                printf "  %-24s %-12s %b  %s\n" "$id" "$agent" "$badge" "$domains"
            done
            ;;
        quotas)
            echo -e "${_C_BCYAN}Quotas${_C_RESET}"
            echo -e "${_C_DIM}$(printf '%.0s─' {1..60})${_C_RESET}"
            curl -sf "$CLAWGRESS_ADMIN_URL/v1/quotas" | jq -r '.[] | "\(.agent_id)\t\(.rps)\t\(.rpm)\t\(.mode)"' | while IFS=$'\t' read -r id rps rpm mode; do
                local badge=$(_clawgress_badge "$mode" "$mode")
                printf "  %-20s RPS:%-6s RPM:%-6s %b\n" "$id" "$rps" "$rpm" "$badge"
            done
            ;;
        audit)
            shift
            /usr/local/sbin/clawgressctl show audit "$@"
            ;;
        conflicts)
            echo -e "${_C_BCYAN}Policy Conflicts${_C_RESET}"
            echo -e "${_C_DIM}$(printf '%.0s─' {1..60})${_C_RESET}"
            local result
            result=$(curl -sf "$CLAWGRESS_ADMIN_URL/v1/policy/conflicts")
            local count
            count=$(echo "$result" | jq -r '.count')
            if [ "$count" = "0" ]; then
                echo -e "  ${_C_GREEN}No conflicts detected${_C_RESET}"
            else
                echo -e "  ${_C_RED}${count} conflict(s) found:${_C_RESET}"
                echo "$result" | jq -r '.conflicts[] | "  \(.rule_a.policy_id) vs \(.rule_b.policy_id) — \(.domain)"'
            fi
            ;;
        config|configuration)
            shift
            /usr/local/sbin/clawgressctl show "$@"
            ;;
        rpz)
            echo -e "${_C_BCYAN}RPZ Zone Preview${_C_RESET}"
            echo -e "${_C_DIM}$(printf '%.0s─' {1..60})${_C_RESET}"
            curl -sf "$CLAWGRESS_ADMIN_URL/v1/rpz/preview"
            ;;
        nft)
            echo -e "${_C_BCYAN}nftables Rules${_C_RESET}"
            echo -e "${_C_DIM}$(printf '%.0s─' {1..60})${_C_RESET}"
            curl -sf "$CLAWGRESS_ADMIN_URL/v1/nft/render"
            ;;
        health)
            local result
            result=$(curl -sf "$CLAWGRESS_ADMIN_URL/healthz" 2>/dev/null)
            if [ $? -eq 0 ]; then
                echo -e "  $(_clawgress_badge ok "HEALTHY") ${_C_GREEN}Gateway and Admin API operational${_C_RESET}"
                local cfg
                cfg=$(curl -sf "$CLAWGRESS_ADMIN_URL/v1/config/validate" 2>/dev/null)
                if [ $? -eq 0 ]; then
                    local agents policies quotas
                    agents=$(echo "$cfg" | jq -r '.agents')
                    policies=$(echo "$cfg" | jq -r '.policies')
                    quotas=$(echo "$cfg" | jq -r '.quotas')
                    echo -e "  ${_C_DIM}Agents: ${agents}  Policies: ${policies}  Quotas: ${quotas}${_C_RESET}"
                fi
            else
                echo -e "  $(_clawgress_badge fail "DOWN") ${_C_RED}Admin API not responding${_C_RESET}"
            fi
            ;;
        version)
            echo -e "${_C_BCYAN}Clawgress${_C_RESET} MVPv1"
            echo -e "${_C_DIM}Debian $(cat /etc/debian_version 2>/dev/null || echo 'unknown')${_C_RESET}"
            echo -e "${_C_DIM}Kernel $(uname -r)${_C_RESET}"
            ;;
        "")
            echo -e "${_C_BCYAN}Usage:${_C_RESET} show <command>"
            echo ""
            echo -e "  ${_C_BOLD}agents${_C_RESET}        List registered agents"
            echo -e "  ${_C_BOLD}policies${_C_RESET}      List policy rules"
            echo -e "  ${_C_BOLD}quotas${_C_RESET}        List rate limits"
            echo -e "  ${_C_BOLD}audit${_C_RESET}         View recent decisions"
            echo -e "  ${_C_BOLD}conflicts${_C_RESET}     Check for policy conflicts"
            echo -e "  ${_C_BOLD}rpz${_C_RESET}           Preview DNS RPZ zone"
            echo -e "  ${_C_BOLD}nft${_C_RESET}           Preview nftables rules"
            echo -e "  ${_C_BOLD}health${_C_RESET}        System health status"
            echo -e "  ${_C_BOLD}version${_C_RESET}       Version info"
            ;;
        *)
            /usr/local/sbin/clawgressctl show "$@"
            ;;
    esac
}

configure() {
    echo -e "${_C_BCYAN}Entering configuration mode${_C_RESET}"
    echo -e "${_C_DIM}Use: set <path> <value>, show, commit, discard, exit${_C_RESET}"
    echo ""
    /usr/local/sbin/clawgressctl configure "$@"
    echo -e "${_C_DIM}Exited configuration mode${_C_RESET}"
}

commit() {
    echo -e "  ${_C_RED}commit${_C_RESET} can only be run from configure mode."
    echo -e "  Run ${_C_BOLD}configure${_C_RESET} first, then ${_C_BOLD}commit${_C_RESET} inside it."
}

set() {
    if [ "$1" = "-e" ] || [ "$1" = "-x" ] || [ "$1" = "+e" ] || [ "$1" = "+x" ] || [ "$1" = "-o" ] || [ "$1" = "+o" ] || [ "$1" = "--" ]; then
        # Pass through bash builtins: set -e, set -x, set -o, etc.
        builtin set "$@"
    else
        echo -e "  ${_C_RED}set${_C_RESET} can only be run from configure mode."
        echo -e "  Run ${_C_BOLD}configure${_C_RESET} first, then ${_C_BOLD}set <path> <value>${_C_RESET} inside it."
    fi
}

discard() {
    echo -e "  ${_C_RED}discard${_C_RESET} can only be run from configure mode."
    echo -e "  Run ${_C_BOLD}configure${_C_RESET} first."
}

install() {
    /usr/local/sbin/clawgressctl install "$@"
}

# Custom PS1 prompt.
if [ "$(whoami)" = "clawgress" ]; then
    export PS1="\[${_C_BCYAN}\]clawgress\[${_C_RESET}\]@\h\[${_C_DIM}\]:\w\[${_C_RESET}\]\$ "
fi

# Export functions so they're available in subshells.
export -f show configure commit set discard install _clawgress_spinner _clawgress_badge
