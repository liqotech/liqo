function main() {
  t=$1
  bash "${BATS_TEST_DIRNAME}"/../../install.sh $t
}

@test "Error Exit with unkown argument" {
  run main not-existing
  [ "$status" -eq 1 ]
}

@test "Help - Ok Exit with --help" {
  run main --help
  [ "$status" -eq 0 ]
}
