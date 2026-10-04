# Sourced by auto-patch-decide.sh and auto-patch-decide_test.sh.
# Keep in sync with inline copies in consumer auto-patch-release CI.

is_harness_only_path() {
  local path="$1"
  [[ "${path}" == lefthook.yml ]] && return 0
  [[ "${path}" == .cursor/* ]] && return 0
  return 1
}

is_packaging_only_path() {
  local path="$1"
  case "${path}" in
    Dockerfile|Dockerfile.*) return 0 ;;
    scripts/docker-*) return 0 ;;
    scripts/ci/docker-*) return 0 ;;
    scripts/ci/switchyard-smoke-routes.toml) return 0 ;;
    .github/workflows/docker-release.yml|.github/workflows/packaging-rebuild.yml) return 0 ;;
  esac
  return 1
}
