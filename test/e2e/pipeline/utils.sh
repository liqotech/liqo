#!/usr/bin/env bash
#shellcheck disable=SC1091

# Define the retry function
waitandretry() {
  local waittime="$1"
  local retries="$2"
  local command="$3"
  local options="$-" # Get the current "set" options

  sleep "${waittime}"

  echo "Running command: ${command} (retries left: ${retries})"

  # Disable set -e
  if [[ $options == *e* ]]; then
    set +e
  fi

  # Run the command, and save the exit code
  $command
  local exit_code=$?

  # restore initial options
  if [[ $options == *e* ]]; then
    set -e
  fi

  # If the exit code is non-zero (i.e. command failed), and we have not
  # reached the maximum number of retries, run the command again
  if [[ $exit_code -ne 0 && $retries -gt 0 ]]; then
    waitandretry "$waittime" $((retries - 1)) "$command"
  else
    # Return the exit code from the command
    return $exit_code
  fi
}
