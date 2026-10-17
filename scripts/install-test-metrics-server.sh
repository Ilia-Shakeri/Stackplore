#!/usr/bin/env bash
set -euo pipefail

# Fixture administration only. Production observation never installs workloads.
if [[ "${STACKPLORE_TEST_KUBE_DISPOSABLE:-}" != 1 || -z "${STACKPLORE_TEST_KUBECONFIG:-}" || ! "${STACKPLORE_TEST_KIND_NAME:-}" =~ ^stackplore-proof-[a-z0-9-]+$ ]]; then
  echo 'Owned disposable Kubernetes fixture guard missing' >&2
  exit 1
fi

kubectl_bin="${STACKPLORE_TEST_KUBECTL:-kubectl}"
context="$("$kubectl_bin" --kubeconfig "$STACKPLORE_TEST_KUBECONFIG" config current-context)"
if [[ "$context" != "kind-$STACKPLORE_TEST_KIND_NAME" ]]; then
  echo 'Fixture context does not match its owned kind cluster' >&2
  exit 1
fi
server="$("$kubectl_bin" --kubeconfig "$STACKPLORE_TEST_KUBECONFIG" config view --minify -o 'jsonpath={.clusters[0].cluster.server}')"
if [[ ! "$server" =~ ^https://127\.0\.0\.1:[0-9]+$ && ! "$server" =~ ^https://\[::1\]:[0-9]+$ ]]; then
  echo 'Fixture API server must use loopback TLS' >&2
  exit 1
fi
kind_bin="${STACKPLORE_TEST_KIND_BIN:-kind}"
nodes="$("$kind_bin" get nodes --name "$STACKPLORE_TEST_KIND_NAME")"
if [[ "$nodes" != "$STACKPLORE_TEST_KIND_NAME-control-plane" ]]; then
  echo 'Owned disposable kind node missing' >&2
  exit 1
fi

fixture_dir="$(mktemp -d)"
trap 'rm -f "$fixture_dir/components.yaml" "$fixture_dir/fixture.yaml"; rmdir "$fixture_dir"' EXIT
curl --fail --silent --show-error --location --retry 2 --connect-timeout 10 --max-time 120 \
  https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.9.0/components.yaml \
  --output "$fixture_dir/components.yaml"
printf '%s  %s\n' 1cec29a5267809306a2c6ec74a3e449abbb705b4a8beed0c8a1963910f72c79b "$fixture_dir/components.yaml" | sha256sum --check --strict -

# kind's kubelet certificate is self-signed. This exception applies only to this
# disposable metrics fixture; the collector still verifies the API server CA.
sed \
  -e '/        - --metric-resolution=15s/a\        - --kubelet-insecure-tls' \
  -e 's|image: registry.k8s.io/metrics-server/metrics-server:v0.9.0$|image: registry.k8s.io/metrics-server/metrics-server:v0.9.0@sha256:d9862115e7c7881280d3d75ca26bda8ffc0fc213315979575bf23ce9826205c0|' \
  "$fixture_dir/components.yaml" > "$fixture_dir/fixture.yaml"
"$kubectl_bin" --kubeconfig "$STACKPLORE_TEST_KUBECONFIG" --request-timeout=30s apply -f "$fixture_dir/fixture.yaml"
"$kubectl_bin" --kubeconfig "$STACKPLORE_TEST_KUBECONFIG" --request-timeout=190s -n kube-system rollout status deployment/metrics-server --timeout=180s
"$kubectl_bin" --kubeconfig "$STACKPLORE_TEST_KUBECONFIG" --request-timeout=190s wait --for=condition=Available apiservice/v1beta1.metrics.k8s.io --timeout=180s
