#!/usr/bin/env bash
# Prints statistics about FirewallConfiguration attach counts.

set -euo pipefail

# Colors (disabled when not a TTY)
if [[ -t 1 ]]; then
  BOLD='\033[1m'
  CYAN='\033[0;36m'
  GREEN='\033[0;32m'
  YELLOW='\033[0;33m'
  RED='\033[0;31m'
  DIM='\033[2m'
  RESET='\033[0m'
else
  BOLD='' CYAN='' GREEN='' YELLOW='' RED='' DIM='' RESET=''
fi

echo -e "${DIM}Fetching FirewallConfigurations...${RESET}"
# Format: namespace/name[:TERMINATING]
fw_list=$(kubectl get firewallconfiguration -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{.metadata.deletionTimestamp}{"\n"}{end}' \
  | awk -F'\t' '{ts=$2; tag=(ts=="" ? "" : ":TERMINATING"); print $1 tag}')

echo -e "${DIM}Fetching FirewallConfigurationAttaches...${RESET}"
# Format: fwConfigName[:TERMINATING]
attach_raw=$(kubectl get firewallconfigurationattachs.networking.liqo.io -A \
  -o jsonpath='{range .items[*]}{.spec.firewallConfigurationRef.name}{"\t"}{.metadata.deletionTimestamp}{"\n"}{end}' 2>/dev/null || true)

if [[ -z "$attach_raw" ]]; then
  attach_raw=$(kubectl get firewallconfigurationattachs.networking.liqo.io -A -o wide --no-headers \
    | awk '{print $NF"\t"}')
fi

# Plain list of referenced fw config names (for counting)
attach_list=$(echo "$attach_raw" | awk -F'\t' '{print $1}')
# Set of attach names that are terminating
attach_terminating=$(echo "$attach_raw" | awk -F'\t' '$2 != "" {print $1}')

total_fw=$(echo "$fw_list" | grep -c '/' || true)
total_attach=$(echo "$attach_list" | grep -vc '^$' || true)
total_fw_terminating=$(echo "$fw_list" | grep -c ':TERMINATING' || true)
total_attach_terminating=$(echo "$attach_terminating" | grep -vc '^$' || true)

# Compute dynamic column width based on longest name (min 38 for the header),
# stripping the :TERMINATING suffix before measuring
col_width=38
while IFS= read -r fw; do
  [[ -z "$fw" ]] && continue
  plain="${fw%%:TERMINATING}"
  (( ${#plain} > col_width )) && col_width=${#plain}
done <<< "$fw_list"

terminating_label_len=15  # visible characters in the label
sep=$(printf '%*s' $(( col_width + 12 + terminating_label_len )) '' | tr ' ' '-')
header="FIREWALLCONFIGURATION (namespace/name)"

echo ""
echo -e "${BOLD}${CYAN}FirewallConfiguration Attach Statistics${RESET}"
echo -e "${DIM}${sep}${RESET}"
printf "${BOLD}%-${col_width}s  %-${terminating_label_len}s  %s${RESET}\n" "$header" "STATUS" "ATTACHES"
echo -e "${DIM}${sep}${RESET}"

max_count=0
max_names=()
zero_count=0

while IFS= read -r fw; do
  [[ -z "$fw" ]] && continue
  is_terminating=0
  plain="${fw%%:TERMINATING}"
  [[ "$fw" == *:TERMINATING ]] && is_terminating=1
  fw_name="${plain##*/}"
  count=$(echo "$attach_list" | grep -cF "$fw_name" || true)
  # Count how many of the referencing attaches are themselves terminating
  attach_term_count=$(echo "$attach_terminating" | grep -cF "$fw_name" || true)
  if (( count == 0 )); then
    count_color=$RED
  elif (( count >= max_count && max_count > 0 )); then
    count_color=$GREEN
  else
    count_color=$YELLOW
  fi
  if (( is_terminating )); then
    status_field="${RED}[TERMINATING]${RESET}"
    name_color=$RED
  else
    status_field="${DIM}-${RESET}"
    name_color=$CYAN
  fi
  count_suffix=""
  (( attach_term_count > 0 )) && count_suffix=" ${RED}(${attach_term_count} terminating)${RESET}"
  printf "${name_color}%-${col_width}s${RESET}  %-${terminating_label_len}b  ${count_color}%d${RESET}%b\n" \
    "$plain" "$status_field" "$count" "$count_suffix"
  if (( count > max_count )); then
    max_count=$count
    max_names=("$plain")
  elif (( count == max_count )); then
    max_names+=("$plain")
  fi
  if (( count == 0 )); then
    (( zero_count++ )) || true
  fi
done <<< "$fw_list"

echo -e "${DIM}${sep}${RESET}"
echo ""
echo -e "${BOLD}Summary:${RESET}"
printf "  ${DIM}%-34s${RESET} ${BOLD}%d${RESET}\n" "Total FirewallConfigurations:" "$total_fw"
printf "  ${DIM}%-34s${RESET} ${BOLD}%d${RESET}\n" "Total FirewallConfigAttaches:" "$total_attach"
printf "  ${DIM}%-34s${RESET} ${RED}${BOLD}%d${RESET}\n" "FirewallConfigs with 0 refs:" "$zero_count"
printf "  ${DIM}%-34s${RESET} ${RED}${BOLD}%d${RESET}\n" "FirewallConfigs terminating:" "$total_fw_terminating"
printf "  ${DIM}%-34s${RESET} ${RED}${BOLD}%d${RESET}\n" "Attaches terminating:" "$total_attach_terminating"
printf "  ${DIM}%-34s${RESET} ${GREEN}${BOLD}%d attaches${RESET}\n" "Most referenced count:" "$max_count"
for name in "${max_names[@]}"; do
  printf "  %-34s ${CYAN}%s${RESET}\n" "" "$name"
done
