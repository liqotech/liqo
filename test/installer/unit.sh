#!/usr/bin/env bats
setup() {
  load "${BATS_TEST_DIRNAME}"/../../install.sh
}

@test "Check protection" {
  run set_variable_from_command
  [ "$status" -eq 1 ]
}

@test "Exit if command is broken (1)" {
  command_to_run="false"
  set -e
  run set_variable_from_command name command_to_run
  [ "$status" -eq 1 ]
}

@test "Exit if command is broken (127)" {
  command_to_run="false-trololo"
  set -e
  run set_variable_from_command name command_to_run
  [ "$status" -eq 127 ]
}

@test "Test overwritten value is maintained" {
  name="ok"
  command="echo ko"
  run set_variable_from_command name command
  [ "$status" -eq 0 ]
  echo $output
  [ "$output" == "[PRE-INSTALL]: name is set to: ok" ]
}

@test "Test default value is correctly assigned" {
  command="echo ok"
  run set_variable_from_command name command
  [ "$status" -eq 0 ]
  echo $output
  [ "$output" == "[PRE-INSTALL]: name is set to: ok" ]
}


