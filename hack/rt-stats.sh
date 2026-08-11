#!/usr/bin/env bash
# Prints statistics about RouteConfiguration binding counts.

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

echo -e "${DIM}Fetching RouteConfigurations...${RESET}"
# Format: namespace/name[:TERMINATING]
rt_list=$(kubectl get routeconfiguration -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{.metadata.deletionTimestamp}{"\n"}{end}' \
  | awk -F'\t' '{ts=$2; tag=(ts=="" ? "" : ":TERMINATING"); print $1 tag}')

echo -e "${DIM}Fetching RouteConfigurationBindings...${RESET}"
# Format: rtConfigName[:TERMINATING]
binding_raw=$(kubectl get routeconfigurationbindings.networking.liqo.io -A \
  -o jsonpath='{range .items[*]}{.spec.routeConfigurationRef.name}{"\t"}{.metadata.deletionTimestamp}{"\n"}{end}' 2>/dev/null || true)

if [[ -z "$binding_raw" ]]; then
  binding_raw=$(kubectl get routeconfigurationbindings.networking.liqo.io -A -o wide --no-headers \
    | awk '{print $NF"\t"}')
fi

# Plain list of referenced rt config names (for counting)
binding_list=$(echo "$binding_raw" | awk -F'\t' '{print $1}')
# Set of binding names that are terminating
binding_terminating=$(echo "$binding_raw" | awk -F'\t' '$2 != "" {print $1}')

total_rt=$(echo "$rt_list" | grep -c '/' || true)
total_binding=$(echo "$binding_list" | grep -vc '^$' || true)
total_rt_terminating=$(echo "$rt_list" | grep -c ':TERMINATING' || true)
total_binding_terminating=$(echo "$binding_terminating" | grep -vc '^$' || true)

# Compute dynamic column width based on longest name (min 38 for the header),
# stripping the :TERMINATING suffix before measuring
col_width=38
while IFS= read -r rt; do
  [[ -z "$rt" ]] && continue
  plain="${rt%%:TERMINATING}"
  (( ${#plain} > col_width )) && col_width=${#plain}
done <<< "$rt_list"

terminating_label_len=15  # visible characters in the label
sep=$(printf '%*s' $(( col_width + 12 + terminating_label_len )) '' | tr ' ' '-')
header="ROUTECONFIGURATION (namespace/name)"

echo ""
echo -e "${BOLD}${CYAN}RouteConfiguration Binding Statistics${RESET}"
echo -e "${DIM}${sep}${RESET}"
printf "${BOLD}%-${col_width}s  %-${terminating_label_len}s  %s${RESET}\n" "$header" "STATUS" "BINDINGS"
echo -e "${DIM}${sep}${RESET}"

max_count=0
max_names=()
zero_count=0

while IFS= read -r rt; do
  [[ -z "$rt" ]] && continue
  is_terminating=0
  plain="${rt%%:TERMINATING}"
  [[ "$rt" == *:TERMINATING ]] && is_terminating=1
  rt_name="${plain##*/}"
  count=$(echo "$binding_list" | grep -cF "$rt_name" || true)
  # Count how many of the referencing bindings are themselves terminating
  binding_term_count=$(echo "$binding_terminating" | grep -cF "$rt_name" || true)
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
  (( binding_term_count > 0 )) && count_suffix=" ${RED}(${binding_term_count} terminating)${RESET}"
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
done <<< "$rt_list"

echo -e "${DIM}${sep}${RESET}"
echo ""
echo -e "${BOLD}Summary:${RESET}"
printf "  ${DIM}%-34s${RESET} ${BOLD}%d${RESET}\n" "Total RouteConfigurations:" "$total_rt"
printf "  ${DIM}%-34s${RESET} ${BOLD}%d${RESET}\n" "Total RouteConfigBindings:" "$total_binding"
printf "  ${DIM}%-34s${RESET} ${RED}${BOLD}%d${RESET}\n" "RouteConfigurations with 0 refs:" "$zero_count"
printf "  ${DIM}%-34s${RESET} ${RED}${BOLD}%d${RESET}\n" "RouteConfigurations terminating:" "$total_rt_terminating"
printf "  ${DIM}%-34s${RESET} ${RED}${BOLD}%d${RESET}\n" "Bindings terminating:" "$total_binding_terminating"
printf "  ${DIM}%-34s${RESET} ${GREEN}${BOLD}%d bindings${RESET}\n" "Most referenced count:" "$max_count"
for name in "${max_names[@]}"; do
  printf "  %-34s ${CYAN}%s${RESET}\n" "" "$name"
done
