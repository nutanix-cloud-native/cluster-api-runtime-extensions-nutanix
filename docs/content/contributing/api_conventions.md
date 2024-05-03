+++
title = "API Conventions"
icon = "fa-solid fa-list-check"
+++

ClusterClass variables in CAREN are defined using standard CRD mechanisms. As such, we follow the upstream [Kubernetes
API conventions]. This page complements and adds to those conventions to ensure CAREN has a consistent and maintainable
API.

## Required vs Optional properties

Every field should be explicitly annotated as optional or required. Being explicit makes it easier for API maintainers
to not have to infer requirements from other details (e.g. `omitempty`, using a pointer field, etc).

### Required properties

Required properties should be annotated with `// +kubebuilder:validation:Required` and should be non-pointer fields. As
fields are non-pointer, zero values would also be acceptable so it is important to add other validation markers if the
zero value is not acceptable. As an example, we can add a required string property that does not accept the empty string
as:

```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
MyProperty string `json:"myProperty"`
```

### Optional properties

Optional properties should be annotated with `// +kubebuilder:validation:Optional` and should generally be pointer
fields (see [String properties](#string-properties) for an exception to that rule). Optional fields should also have
validation properties to ensure that if they are set they conform to the required validation rules. As an example:

```go
// +kubebuilder:validation:Optional
// +kubebuilder:validation:MinLength=1
MyProperty string `json:"myProperty,omitempty"`
```

## Validation markers

Ensure that fields have as strict validation as possible by using the [Kubebuilder CRD validation markers].

We currently use an unreleased version of [controller-gen] which includes markers for slices. All of the referenced
markers above are also valid for array items by using `// +kubebuilder:validation:items:` prefix, e.g. `//
+kubebuilder:validation:items:Pattern=^a.+$`.

## String properties

API conventions recommend using pointer values for optional fields to be able to distinguish between unset and empty
(`nil` pointer meaning unset, `""` meaning set explicitly to empty string). In almost cases in the CAREN API, an empty
string actually implies unset so a string pointer is unnecessary. If an empty string is an acceptable value for an
optional property, then a pointer should be used to distinguish between unset and empty.

### Formats

Using property format definitions (via the `// +kubebuilder:validation:Format` marker) provides a simple and powerful
way to define complex formats, avoiding the need for overly complex regular expression patterns, and as such should be
used wherever possible. Please refer to the [API extensions formats] that are supported by Kubernetes.

Formats can be combined with other validation rules, e.g. patterns, to provide powerful and strict validation.

[Kubernetes API conventions]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
[Kubebuilder CRD validation markers]: https://book.kubebuilder.io/reference/markers/crd-validation
[API extensions formats]: https://github.com/kubernetes/apiextensions-apiserver/blob/v0.30.0/pkg/apiserver/validation/formats.go#L26-L51
