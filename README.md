# Thanos Operator

> [!NOTE]  
> This operator is in very early stages of development. Proceed with caution.

Operator to manage Thanos installations.

## Testing

The following `make` targets are available for testing:

1. `make test` - Runs unit and integration tests.
2. `make test-e2e` - Runs e2e tests against a Kubernetes cluster.

When executing integration tests, the following environment variables can be used to skip specific, per-controller tests:
* EXCLUDE_COMPACT=true 
* EXCLUDE_QUERY=true 
* EXCLUDE_RULER=true 
* EXCLUDE_RECEIVE=true
* EXCLUDE_STORE=true

As an example, to run only integration tests for ThanosStore, you can run the following command:
```bash
EXCLUDE_COMPACT=true EXCLUDE_QUERY=true EXCLUDE_RULER=true EXCLUDE_RECEIVE=true make test
```
