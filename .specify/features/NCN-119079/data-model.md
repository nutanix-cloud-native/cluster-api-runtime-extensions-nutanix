<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Data Model: Support http schemes for container registries

## Entities

### ContainerdConfig (internal)

Existing struct in `pkg/handlers/generic/mutation/generic/mirrors/containerd_files.go`:

```go
type containerdConfig struct {
  URL          string // Registry URL with scheme (http:// or https://), may include path
  CASecretName string // Name of secret containing ca.crt, if present
  CACert       string // Content of ca.crt, if present
  Mirror       bool   // True for GlobalImageRegistryMirror, false for ImageRegistry
}
```

Extended with helpers:

- `isHTTP() bool` – parses URL, returns true if scheme == "http"
- `needContainerdConfiguration() bool` – now `CACert != "" || Mirror || isHTTP()`

### PerHostEntry (internal, transient)

```go
type perHostEntry struct {
  host      string // URL.Host (host[:port])
  serverURL string // "http://<host>"
  hostURL   string // "http://<host>" (same as serverURL, path excluded)
}
```

Used only during `generateContainerdPerHostHostsFiles` to deduplicate by host.

### API Types (unchanged)

- `GlobalImageRegistryMirror` – `URL string` validated `^https?://`, `Credentials *RegistryCredentials`
- `ImageRegistry` – same validation
- Both already allow http and https; no model change.

## Relationships

- One Cluster variable `globalImageRegistryMirror` (optional, 0..1) → one `containerdConfig` with `Mirror=true`
- Zero or more `imageRegistries` (0..32) → each one `containerdConfig` with `Mirror=false`
- Multiple configs may share same `URL.Host` → deduplicated to single `ca.crt` file and/or single per-host `hosts.toml`

## Validation

- API: `kubebuilder:validation:Pattern=^https?://` + `Format=uri`
- Handler: `ErrConflictingRegistryCACertificates` if same host has differing `CACert` contents
- New: http imageRegistries without CA still trigger file generation (previously ignored)

## File Generation Mapping

| Input | Scheme | Mirror | Generated Files |
|-------|--------|--------|-----------------|
| GlobalMirror http | http | true | `_default/hosts.toml` (http URL), `registry-config.toml`, optional `ca.crt` |
| GlobalMirror https | https | true | `_default/hosts.toml` (https URL), `registry-config.toml`, optional `ca.crt` |
| ImageRegistry http | http | false | `<host>/hosts.toml` (http), `registry-config.toml`, optional `ca.crt` |
| ImageRegistry https with CA | https | false | `<host>/ca.crt`, `registry-config.toml` |
| ImageRegistry https no CA | https | false | (none) – existing behavior, no change |
