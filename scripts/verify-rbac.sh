#!/usr/bin/env bash

set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
chart="$repo_root/deploy/charts/prefect-operator"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

cd "$repo_root"

controller-gen \
  rbac:roleName=prefect-operator \
  paths="./..." \
  output:rbac:artifacts:config="$tmp_dir/generated"

helm dependency build "$chart" >/dev/null
helm template prefect-operator "$chart" \
  --show-only templates/rbac.yaml >"$tmp_dir/rendered.yaml"

expand_rules() {
  local expression=$1
  local manifest=$2

  yq -r "$expression" "$manifest" | sort -u
}

rule_expression=".rules[] | .apiGroups[] as \$group | .resources[] as \$resource | .verbs[] | [\$group, \$resource, .] | @tsv"
rendered_rule_expression="select(.kind == \"ClusterRole\") | .rules[] | .apiGroups[] as \$group | .resources[] as \$resource | .verbs[] | [\$group, \$resource, .] | @tsv"

missing_rules=$(
  comm -23 \
    <(expand_rules "$rule_expression" "$tmp_dir/generated/role.yaml") \
    <(expand_rules "$rendered_rule_expression" "$tmp_dir/rendered.yaml")
)

if [[ -n "$missing_rules" ]]; then
  echo "The Helm ClusterRole is missing controller RBAC rules:"
  echo "$missing_rules"
  exit 1
fi
