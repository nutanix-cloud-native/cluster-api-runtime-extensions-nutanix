<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Consolidate webhook servers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve CAPI runtime extension hooks and Kubernetes admission webhooks on one controller-runtime HTTPS listener (port 9443) with one TLS cert, without changing handler behavior.

**Architecture:** Use CAPI `*runtimeserver.Server` as the manager `WebhookServer`. That type embeds `webhook.Server` and its `Start` registers runtime hook paths then listens once. Register admission handlers on the same instance via `Register` before `mgr.Start`. Do not `mgr.Add` a second HTTPS runnable. Chart keeps two Services pointing at the shared port; one Certificate (`*-runtimehooks-tls`) with SANs for both Service DNS names.

**Tech Stack:** Go, controller-runtime v0.22.5, CAPI `exp/runtime/server` v1.12.x, Helm chart, cert-manager Certificates, testify.

**Branch**: `NCN-116148-consolidate-webhook-servers`
**Date**: 2026-07-24
**Spec**: [./spec.md](./spec.md)

## Global Constraints

- No topology mutation handler version bumps; `GeneratePatches` output must stay identical (FR-013).
- No changes to admission or runtime handler business logic (FR-004, FR-005).
- Keep Service names `*-runtimehooks` and `*-admission` (FR-006).
- Remove `--admission-webhook-cert-dir` entirely — no alias (FR-012).
- Prefer `devbox run --` for Go/Helm commands when `devbox.json` is present.
- Do not commit unless the user explicitly asks (or the execution session has commit approval).

---

## Summary

Today `cmd/main.go` creates a manager `WebhookServer` on 9444 with `/admission-certs`, then `mgr.Add`s a separate `common/pkg/server` runnable that starts CAPI `runtimeserver` on 9443 with `/runtimehooks-certs`. Both are path-routed HTTPS; paths do not collide. This plan collapses them onto one listener by making the CAPI runtime server the manager webhook server, then updates chart TLS/ports/flags accordingly.

## Technical Context

**Language/Version**: Go (see `go.mod`).
**Primary Dependencies**: `sigs.k8s.io/controller-runtime`, `sigs.k8s.io/cluster-api/exp/runtime/server`.
**Testing**: testify unit tests in `common/pkg/server`; Helm template checks via `helm template` + `yq`; existing admission webhook suite tests remain valid.
**Target Platform**: management-cluster Deployment.
**Project Type**: single Go module + Helm chart.
**Constraints**: single HTTPS port 9443; single cert secret `*-runtimehooks-tls`; handler version safety.
**Scale/Scope**: `common/pkg/server`, `cmd/main.go`, chart templates, committed `runtime-extensions-components.yaml` for local e2e.

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| I. API-First | Pass | No API/CRD changes. |
| II. Handler-per-Provider | Pass | No handler package changes. |
| III. Library-First | Pass | Wiring stays in `common/pkg/server` + thin `cmd`. |
| IV. Tests Required | Pass | New server unit test + chart render checks. |
| V. Code Style | Pass | Follow existing import aliases; no narrating comments. |
| VI. Dependency Management | Pass | No new deps. |
| VII. Handler Version Safety | Pass | No `pkg/handlers/*/mutation/` edits. |
| VIII. Handler Documentation | Pass | No handler behavior docs required; optional deploy note only if docs mention dual ports/certs. |

No violations.

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-116148/
├── plan.md
└── spec.md
```

### Source Code (repository root)

```text
common/pkg/server/
├── server.go              # Create runtimeserver; AddHandlers; no second listener Runnable
└── server_test.go         # Shared-server registration + dual-path smoke test

cmd/
└── main.go                # Use runtime server as mgr WebhookServer; drop admission cert flag/port

charts/cluster-api-runtime-extensions-nutanix/templates/
├── certificates.yaml      # SANs for both Services; delete admission Certificate
├── deployment.yaml        # One port, one cert mount/arg
├── admission-service.yaml # targetPort → shared HTTPS port name
├── runtimehooks-service.yaml  # unchanged name; same targetPort
└── webhooks.yaml          # inject-ca-from → runtimehooks-tls

runtime-extensions-components.yaml  # Regenerate from helm template for local e2e
```

**Structure Decision:** Keep the existing `common/pkg/server` package as the place that builds/registers CAPI runtime handlers. Change its API so callers obtain a `webhook.Server` suitable for `manager.Options.WebhookServer`, then call `AddHandlers` after the manager (and thus handlers) exist.

## Key Wiring (read before coding)

Chicken-and-egg: `AllHandlers(mgr)` needs a manager, but `WebhookServer` must be set in `manager.Options` before `NewManager`.

```text
1. Parse flags (--webhook-port default 9443, --webhook-cert-dir)
2. runtimeSrv := server.NewWebhookServer(opts)          // catalog + runtimeserver.New; NO Start
3. mgr := NewManager(..., WebhookServer: runtimeSrv)
4. handlers := AllHandlers(mgr)
5. server.AddHandlers(runtimeSrv, handlers...)          // AddExtensionHandler only
6. mgr.GetWebhookServer().Register(/mutate-..., ...)  // same instance
7. mgr.Start(ctx)  // → runtimeserver.Start → register hook paths → listen once
```

Do **not** call `mgr.Add(runtimeSrv)` separately: `GetWebhookServer()` already `Add`s the webhook server runnable.

`*runtimeserver.Server` implements `webhook.Server` (embeds the interface and defines `Start` that registers hooks then listens). Manager type-switch treats it as `webhook.Server` and runs that `Start`.

## Tasks

Atomic, TDD-ordered. Do not merge a task until its acceptance check passes.

### T1 — Refactor `common/pkg/server` API (failing tests first)

**Files:**

- Create: `common/pkg/server/server_test.go`
- Modify: `common/pkg/server/server.go`

**Interfaces:**

- Produces:
  - `func NewServerOptions() *ServerOptions` (keep; default `webhookPort` to `9443`)
  - `func (o *ServerOptions) AddFlags(*pflag.FlagSet)` — flags `--webhook-port`, `--webhook-cert-dir` only
  - `func NewWebhookServer(opts *ServerOptions) (webhook.Server, error)` — builds catalog + `runtimeserver.New`
  - `func AddHandlers(s webhook.Server, hooks ...handlers.Named) error` — type-asserts to `*runtimeserver.Server` (or accept `*runtimeserver.Server`), runs the existing `AddExtensionHandler` loop from today’s `Start`
- Removes: custom `Server` Runnable with `Start`/`NeedLeaderElection` that creates a second listener

- [ ] **Step 1: Write failing tests**

Create `common/pkg/server/server_test.go` in package `server` (same package) so tests can set unexported option fields:

```go
package server

import (
  "context"
  "crypto/ecdsa"
  "crypto/elliptic"
  "crypto/rand"
  "crypto/tls"
  "crypto/x509"
  "crypto/x509/pkix"
  "encoding/pem"
  "fmt"
  "io"
  "math/big"
  "net"
  "net/http"
  "os"
  "path/filepath"
  "testing"
  "time"

  "github.com/spf13/pflag"
  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
  "sigs.k8s.io/controller-runtime/pkg/webhook"
  "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type namedStub struct{}

func (namedStub) Name() string { return "stub" }

func TestNewWebhookServer_ImplementsWebhookServer(t *testing.T) {
  t.Parallel()
  opts := NewServerOptions()
  opts.webhookPort = 0
  opts.webhookCertDir = t.TempDir()
  ws, err := NewWebhookServer(opts)
  require.NoError(t, err)
  require.NotNil(t, ws)
  _, ok := any(ws).(webhook.Server)
  assert.True(t, ok)
}

func TestAddHandlersAndAdmissionShareListener(t *testing.T) {
  t.Parallel()

  certDir := t.TempDir()
  require.NoError(t, writeTestServingCerts(certDir))

  ln, err := net.Listen("tcp", "127.0.0.1:0")
  require.NoError(t, err)
  port := ln.Addr().(*net.TCPAddr).Port
  require.NoError(t, ln.Close())

  opts := NewServerOptions()
  opts.webhookPort = port
  opts.webhookCertDir = certDir

  ws, err := NewWebhookServer(opts)
  require.NoError(t, err)
  require.NoError(t, AddHandlers(ws, namedStub{}))

  ws.Register("/mutate-test", &webhook.Admission{
    Handler: admission.HandlerFunc(func(_ context.Context, _ admission.Request) admission.Response {
      return admission.Allowed("")
    }),
  })

  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()
  errCh := make(chan error, 1)
  go func() { errCh <- ws.Start(ctx) }()

  client := &http.Client{
    Transport: &http.Transport{
      TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test-only
    },
  }
  require.Eventually(t, func() bool {
    resp, err := client.Post(
      fmt.Sprintf("https://127.0.0.1:%d/hooks.runtime.cluster.x-k8s.io/v1alpha1/discovery", port),
      "application/json",
      nil,
    )
    if err != nil {
      return false
    }
    defer resp.Body.Close()
    _, _ = io.Copy(io.Discard, resp.Body)
    return resp.StatusCode == http.StatusOK
  }, 10*time.Second, 50*time.Millisecond)

  resp, err := client.Post(
    fmt.Sprintf("https://127.0.0.1:%d/mutate-test", port),
    "application/json",
    nil,
  )
  require.NoError(t, err)
  defer resp.Body.Close()
  // Admission may 400 on empty body; connection + TLS + route hit is enough.
  assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

  cancel()
  select {
  case err := <-errCh:
    require.NoError(t, err)
  case <-time.After(5 * time.Second):
    t.Fatal("server did not shut down")
  }
}

func TestServerOptions_NoAdmissionCertDirFlag(t *testing.T) {
  t.Parallel()
  fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
  NewServerOptions().AddFlags(fs)
  assert.Nil(t, fs.Lookup("admission-webhook-cert-dir"))
  assert.NotNil(t, fs.Lookup("webhook-port"))
  assert.NotNil(t, fs.Lookup("webhook-cert-dir"))
}

func writeTestServingCerts(dir string) error {
  key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
  if err != nil {
    return err
  }
  tmpl := &x509.Certificate{
    SerialNumber: big.NewInt(1),
    Subject:      pkix.Name{CommonName: "localhost"},
    NotBefore:    time.Now().Add(-time.Hour),
    NotAfter:     time.Now().Add(24 * time.Hour),
    KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
    ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
    DNSNames:     []string{"localhost"},
  }
  der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
  if err != nil {
    return err
  }
  certOut, err := os.Create(filepath.Join(dir, "tls.crt"))
  if err != nil {
    return err
  }
  defer certOut.Close()
  if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
    return err
  }
  keyBytes, err := x509.MarshalECPrivateKey(key)
  if err != nil {
    return err
  }
  keyOut, err := os.Create(filepath.Join(dir, "tls.key"))
  if err != nil {
    return err
  }
  defer keyOut.Close()
  return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
}
```

- [ ] **Step 2: Run tests — expect compile/fail**

```bash
devbox run -- go test ./common/pkg/server/ -count=1
```

Expected: fail — `NewWebhookServer` / `AddHandlers` undefined (or Runnable `Start` still owns the listener).

- [ ] **Step 3: Implement minimal API**

Rewrite `common/pkg/server/server.go` roughly as:

```go
func NewServerOptions() *ServerOptions {
  return &ServerOptions{webhookPort: 9443}
}

func NewWebhookServer(opts *ServerOptions) (webhook.Server, error) {
  catalog := runtimecatalog.New()
  _ = runtimehooksv1.AddToCatalog(catalog)
  return runtimeserver.New(runtimeserver.Options{
    Catalog: catalog,
    Port:    opts.webhookPort,
    CertDir: opts.webhookCertDir,
  })
}

func AddHandlers(s webhook.Server, hooks ...handlers.Named) error {
  rs, ok := s.(*runtimeserver.Server)
  if !ok {
    return fmt.Errorf("webhook server is %T, want *runtimeserver.Server", s)
  }
  for _, h := range hooks {
    // Copy the existing type-switch + AddExtensionHandler body from the old
    // Server.Start loop verbatim (BeforeClusterCreate … ValidateTopology).
    // Do NOT call rs.Start here — manager owns Start.
    _ = h
  }
  return nil
}
```

When implementing, paste the full `for _, h := range s.hooks { … }` block from the current `Server.Start` into `AddHandlers` (changing `s.hooks` to the `hooks` argument and `webhookServer` to `rs`). Delete the old `Server` struct Runnable (`Start` / `NeedLeaderElection`) that constructed and started a second server.

- [ ] **Step 4: Re-run tests — expect pass**

```bash
devbox run -- go test ./common/pkg/server/ -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add common/pkg/server/
git commit -m "$(cat <<'EOF'
refactor: [NCN-116148] Expose shared runtime webhook.Server API

Allow the CAPI runtime server to be used as the manager WebhookServer
so admission and runtime hooks can share one listener.
EOF
)"
```

**Acceptance:** `go test ./common/pkg/server` passes; no second-listener Runnable remains in the package.

---

### T2 — Wire `cmd/main.go` onto the shared server

**Files:**

- Modify: `cmd/main.go`
- Test: `common/pkg/server` (already covers dual paths); compile `cmd`

**Interfaces:**

- Consumes: `server.NewWebhookServer`, `server.AddHandlers`, `server.NewServerOptions`
- Produces: process with one HTTPS listener; flags without `--admission-webhook-cert-dir`

- [ ] **Step 1: Write a failing flag assertion**

Add a small test under `cmd` only if the package is testable without `main` hazards; otherwise extend `TestServerOptions_NoAdmissionCertDirFlag` and add a compile-time check by deleting the admission flag usages (Step 3). Prefer not inventing a heavy `cmd` test.

- [ ] **Step 2: Change manager construction**

Replace:

```go
webhookOptions := webhook.Options{Port: 9444, CertDir: "/admission-certs"}
mgrOptions := &ctrl.Options{
  WebhookServer: webhook.NewServer(webhookOptions),
  // ...
}
// --admission-webhook-cert-dir flag
runtimeWebhookServer := server.NewServer(...)
mgr.Add(runtimeWebhookServer)
```

With:

```go
runtimeWebhookServerOpts := server.NewServerOptions()
// default cert dir for local/dev if desired:
// still set via flag; chart passes --webhook-cert-dir=/runtimehooks-certs/

runtimeWebhookServerOpts.AddFlags(pflag.CommandLine)
// ... parse flags ...

webhookServer, err := server.NewWebhookServer(runtimeWebhookServerOpts)
if err != nil { /* exit */ }

mgrOptions := &ctrl.Options{
  WebhookServer: webhookServer,
  // Metrics/HealthProbe unchanged
}
mgr, err := newManager(mgrOptions)

allHandlers := slices.Concat(...)
if err := server.AddHandlers(webhookServer, allHandlers...); err != nil { /* exit */ }

// Admission registrations unchanged — they use mgr.GetWebhookServer()
mgr.GetWebhookServer().Register("/mutate-cluster", ...)
// ...
// Do NOT mgr.Add(webhookServer) again.
mgr.Start(signalCtx)
```

Remove `--admission-webhook-cert-dir` flag registration and any `/admission-certs` defaults.

Ensure `WebhookServer` is assigned **before** `newManager`, and `AddHandlers` runs **after** `AllHandlers(mgr)`.

- [ ] **Step 3: Build**

```bash
devbox run -- go build -o /dev/null ./cmd/
```

Expected: success.

- [ ] **Step 4: Confirm help has no admission cert flag**

```bash
devbox run -- go run ./cmd --help 2>&1 | tee /tmp/caren-help.txt
grep -E 'webhook-port|webhook-cert-dir|admission-webhook' /tmp/caren-help.txt
```

Expected: `webhook-port` and `webhook-cert-dir` present; `admission-webhook-cert-dir` absent.

- [ ] **Step 5: Commit** (only if user asked)

```bash
git add cmd/main.go
git commit -m "$(cat <<'EOF'
refactor: [NCN-116148] Serve admission on the runtime webhook server

Use a single manager WebhookServer for runtime hooks and admission
paths so upgrading CAREN does not require two TLS listeners.
EOF
)"
```

**Acceptance:** Binary builds; one webhook server in process wiring; admission flag gone.

---

### T3 — Chart: certificate SANs + CA inject

**Files:**

- Modify: `charts/cluster-api-runtime-extensions-nutanix/templates/certificates.yaml`
- Modify: `charts/cluster-api-runtime-extensions-nutanix/templates/webhooks.yaml`

- [ ] **Step 1: Update Certificate**

In `certificates.yaml`, keep only `*-runtimehooks-tls` and expand `dnsNames`:

```yaml
dnsNames:
  - {{ template "chart.name" . }}-runtimehooks.{{ .Release.Namespace }}.svc
  - {{ template "chart.name" . }}-runtimehooks.{{ .Release.Namespace }}.svc.cluster.local
  - {{ template "chart.name" . }}-admission.{{ .Release.Namespace }}.svc
  - {{ template "chart.name" . }}-admission.{{ .Release.Namespace }}.svc.cluster.local
```

Delete the entire `*-admission-tls` Certificate document.

- [ ] **Step 2: Point admission webhooks at runtimehooks cert**

In `webhooks.yaml`, change both `cert-manager.io/inject-ca-from` values from `...-admission-tls` to `...-runtimehooks-tls`.

- [ ] **Step 3: Render and assert**

```bash
devbox run -- helm template caren ./charts/cluster-api-runtime-extensions-nutanix \
  --namespace caren-system > /tmp/caren-chart.yaml
yq 'select(.kind == "Certificate") | .metadata.name' /tmp/caren-chart.yaml
yq 'select(.kind == "Certificate") | .spec.dnsNames' /tmp/caren-chart.yaml
yq 'select(.kind == "MutatingWebhookConfiguration" or .kind == "ValidatingWebhookConfiguration") | .metadata.annotations["cert-manager.io/inject-ca-from"]' /tmp/caren-chart.yaml
```

Expected: one Certificate name ending in `runtimehooks-tls`; dnsNames include both Services; webhook annotations reference `runtimehooks-tls` only; no `admission-tls` Certificate.

- [ ] **Step 4: Commit** (only if user asked)

**Acceptance:** FR-007, FR-008, FR-009 satisfied in rendered output.

---

### T4 — Chart: Deployment + Services share one port

**Files:**

- Modify: `charts/cluster-api-runtime-extensions-nutanix/templates/deployment.yaml`
- Modify: `charts/cluster-api-runtime-extensions-nutanix/templates/admission-service.yaml`
- Verify: `charts/cluster-api-runtime-extensions-nutanix/templates/runtimehooks-service.yaml` (name unchanged)
- Verify: `charts/cluster-api-runtime-extensions-nutanix/templates/extensionconfig.yaml` (unchanged service + CA secret)

- [ ] **Step 1: Deployment**

- Keep `--webhook-cert-dir=/runtimehooks-certs/`.
- Remove `--admission-webhook-cert-dir=...`.
- Ports: keep single HTTPS `containerPort: 9443` name `runtimehooks` (or rename to `https` — if renamed, update both Services). Remove `9444` / `admission` port.
- Volumes: only `runtimehooks-cert` secret `*-runtimehooks-tls`. Remove `admission-cert` volume/mount.

- [ ] **Step 2: Admission Service**

Change `targetPort: admission` → `targetPort: runtimehooks` (or `https` if renamed). Keep Service metadata name `*-admission`.

- [ ] **Step 3: Render and assert**

```bash
devbox run -- helm template caren ./charts/cluster-api-runtime-extensions-nutanix \
  --namespace caren-system > /tmp/caren-chart.yaml
yq 'select(.kind == "Deployment") | .spec.template.spec.containers[0].ports[].containerPort' /tmp/caren-chart.yaml
yq 'select(.kind == "Deployment") | .spec.template.spec.containers[0].args[]' /tmp/caren-chart.yaml | rg 'webhook|admission' || true
yq 'select(.kind == "Service") | {"name": .metadata.name, "target": .spec.ports[0].targetPort}' /tmp/caren-chart.yaml
```

Expected:

- Container ports include 9443 once; no 9444.
- No `--admission-webhook-cert-dir`.
- Services `...-runtimehooks` and `...-admission` both target the same port name.
- ExtensionConfig still references `...-runtimehooks` and `runtimehooks-tls` CA annotation.

- [ ] **Step 4: Commit** (only if user asked)

**Acceptance:** FR-002, FR-006, FR-010, FR-011, SC-001, SC-002.

---

### T5 — Regenerate `runtime-extensions-components.yaml`

**Files:**

- Modify: `runtime-extensions-components.yaml` (committed install manifest used by local e2e)

- [ ] **Step 1: Regenerate from chart**

Mirror goreleaser’s helm template (dev tag is fine for local tree):

```bash
devbox run -- sh -ec '
NS=caren-system
PROJECT=cluster-api-runtime-extensions-nutanix
{
  cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NS}
EOF
  helm template "${PROJECT}" "./charts/${PROJECT}" --namespace "${NS}"
} > runtime-extensions-components.yaml
'
```

- [ ] **Step 2: Spot-check**

```bash
rg -n 'admission-tls|9444|admission-webhook-cert-dir|containerPort: 9443' runtime-extensions-components.yaml
```

Expected: no `admission-tls` / `9444` / `admission-webhook-cert-dir`; `9443` present; both Service names remain.

- [ ] **Step 3: Commit** (only if user asked)

**Acceptance:** Local e2e manifest matches chart consolidation.

---

### T6 — Final verification (no mutation churn)

**Files:** review-only (no mutation handler edits)

- [ ] **Step 1: Diff touch list**

```bash
git diff --name-only
```

Expected: no files under `pkg/handlers/*/mutation/` or `pkg/handlers/v5/`.

- [ ] **Step 2: Unit tests for touched packages**

```bash
devbox run -- go test ./common/pkg/server/ ./cmd/ ./pkg/webhook/... -count=1
```

Expected: PASS (skip packages that cannot build in isolation if `cmd` has no tests).

- [ ] **Step 3: Optional representative patch sanity**

If an existing capitest/GeneratePatches test target is cheap:

```bash
devbox run -- go test ./pkg/handlers/generic/mutation/... -count=1 -timeout 10m
```

Expected: PASS — confirms no accidental handler edits. (Not a before/after byte compare; this refactor must not change those packages.)

- [ ] **Step 4: Docs grep**

```bash
rg -n '9444|admission-certs|admission-tls|admission-webhook-cert-dir' docs/ || true
```

Update any user-facing deploy docs that document dual ports/certs. Skip if none.

**Acceptance:** FR-013 / SC-004 satisfied by inspection + green tests; SC-005 already covered in T2.

---

## Spec coverage checklist

| Spec item | Task |
| --- | --- |
| FR-001 / FR-003 single listener | T1, T2 |
| FR-002 port 9443 | T2, T4 |
| FR-004 / FR-005 paths/behavior unchanged | T2 (Register paths unchanged), T1 smoke |
| FR-006 two Services same targetPort | T4 |
| FR-007 / FR-008 single Certificate | T3 |
| FR-009 CA inject annotation | T3 |
| FR-010 ExtensionConfig unchanged service/CA secret name | T4 verify |
| FR-011 Deployment single mount/port/arg | T4 |
| FR-012 remove admission cert flag | T1, T2 |
| FR-013 no mutation version bump | T6 |
| SC-001–SC-005 | T2–T6 |

## Out of scope (do not implement)

- Upstream CAPI API to inject an existing `webhook.Server` into `runtimeserver.New`
- Merging the two Services
- Metrics/health port consolidation
