# Preflight Checks Framework

The preflight checks framework is a validating admission webhook that runs a series of checks before a `Cluster` resource is created. It helps ensure that a cluster's configuration is valid, and that the underlying infrastructure is ready, preventing common issues before they occur.

The framework is designed to be extensible, allowing different sets of checks to be grouped into logical units called `Checker`s.

## Core Concepts

The framework is built around a few key interfaces and structs:

- **`preflight.WebhookHandler`**: The main entry point for the webhook. It receives admission requests, decodes the `Cluster` object, and orchestrates the execution of all registered `Checker`s.

- **`preflight.Checker`**: A collection of checks, logically related to some external API, and sharing dependencies, such as a client for the external API. Each `Checker` is responsible for initializing and returning a slice of `Check`s to be executed. At the time of this writing, we have two checkers:
  - `generic.Checker`: For checks that are not specific to any infrastructure provider.
  - `nutanix.Checker`: For checks specific to the Nutanix infrastructure. All the checks share a Prism Central API client.

- **`preflight.Check`**: Represents a single, atomic validation. Each check must implement two methods:
  - `Name() string`: Returns a unique name for the check. This name is used for identification, and for skipping checks.
  - `Run(ctx context.Context) CheckResult`: Executes the validation logic.

- **`preflight.CheckResult`**: The outcome of a `Check`. It indicates if the check was `Allowed`, if an `InternalError` occurred, and provides a list of `Causes` for failure and any `Warnings`.

### Create a Checker

#### Implement a new Go package

Create a new package for your checker under the preflight directory. For example, `pkg/webhook/preflight/myprovider/`.

Create a checker.go file to define your `Checker`. This checker will initialize all the checks for your provider. A common pattern is to have a configuration pseudo-check that runs first, parses provider-specific configuration, initializes an API client, and then initialize checks with the configuration and client.

````go
package myprovider

import (
    "context"

    clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
    ctrl "sigs.k8s.io/controller-runtime"
    ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

    "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

// Expose the checker as a package variable.
var Checker = &myChecker{
    // Use factories to create checks.
}

type myChecker struct {
    // factories for creating checks
}

// checkDependencies holds shared data for all checks.
type checkDependencies struct {
    // provider-specific config, clients, etc.
}

func (m *myChecker) Init(
    ctx context.Context,
    client ctrlclient.Client,
    cluster *clusterv1.Cluster,
) []preflight.Check {
    log := ctrl.LoggerFrom(ctx).WithName("preflight/myprovider")

    cd := &checkDependencies{
        // initialize dependencies
    }

    checks := []preflight.Check{
        // It's good practice to have a configuration check run first.
        newConfigurationCheck(cd),
    }

    // Add other checks
    checks = append(checks, &myCheck{})

    return checks
}
````

The `generic.Checker` and `nutanix.Checker` serve as excellent reference implementations. The `nutanix.Checker` demonstrates a more complex setup with multiple dependent checks.

#### Register the Checker

Finally, add your new `Checker` to the list of checkers in main.go.

````go
// ...existing code...
import (
 preflightgeneric "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/generic"
 preflightnutanix "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/nutanix"
 preflightmyprovider "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/myprovider"
)
// ...existing code...
 if err := mgr.Add(preflight.New(
  mgr.GetClient(),
  mgr.GetWebhookServer().
   GetAdmissionDecoder(),
  []preflight.Checker{
   // Add your preflight checkers here.
   preflightgeneric.Checker,
   preflightnutanix.Checker,
   preflightmyprovider.Checker,
  }...,
 )); err != nil {
// ...existing code...
````

## Create a Check

Create a struct that implements the `preflight.Check` interface.

The `Name` method should return a concise, unique name that is a combination of the Checker and Check name, e.g. `NutanixVMImage`. Checks in the `generic` Checker only use the Check name, e.g. `Registry. The name is used to identify,and skip checks.

The `Run` method should return a `CheckResult` that indicates whether the check passed (`Allowed`), whether an internal error occurred (`InternalError`), and one or more `Causes`es, each including a `Message` that explains why the check failed, and a `Field` that points the user to the configuration that should be examined and possibly changed.

If a check runs to completion, then `InternalError` should be false. It should be `true` only in case of an _unexpected_ error, such as a malformed response from some API.

If the check passes, then `Allowed` should be `true`.

The `Message` should include context to help the user understand the problem, and the most common ways to help them resolve the problem. Even so, the message should be concise, as it will be displayed in the CLI and UI clients.

The `Field` should be a valid JSONPath expression that identifies the most relevant part of the Cluster configuration. Look at existing checkers for examples.

````go
package myprovider

import (
    "context"
    "fmt"

    "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type myCheck struct {
    // any dependencies the check needs
}

func (c *myCheck) Name() string {
    return "MyProviderCheck"
}

func (c *myCheck) Run(ctx context.Context) preflight.CheckResult {
    // Your validation logic here.
    // For example, check a specific condition.
    isValid := true // replace with real logic
    if !isValid {
        return preflight.CheckResult{
            Allowed: false,
            Causes: []preflight.Cause{
                {
                    Message: "My custom check failed because of a specific reason.",
                    Field:   "$.spec.topology.variables[?@.name=='myProviderConfig']",
                },
            },
        }
    }

    return preflight.CheckResult{Allowed: true}
}
````
