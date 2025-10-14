# Requirements Document

## Introduction

The ExternalSource Controller is a Kubernetes operator built using the Kubebuilder framework that integrates external, non-Git data sources into the GitOps workflow powered by Flux. The controller periodically fetches data from HTTP APIs, optionally transforms it, packages it as versioned artifacts, and makes it available for consumption by other Flux controllers through an ExternalArtifact custom resource. This enables reliable, scalable, and observable management of dynamic configuration data within Kubernetes environments.

## Requirements

### Requirement 1

**User Story:** As a platform engineer, I want to define external data sources through a Kubernetes custom resource with pluggable source types, so that I can integrate various non-Git configuration data sources into my GitOps workflow.

#### Acceptance Criteria

1. WHEN an ExternalSource custom resource is created THEN the system SHALL validate the resource specification according to the CRD schema
2. WHEN the ExternalSource specifies a generator type THEN the system SHALL use a modular source interface to handle different source implementations
3. WHEN HTTP is specified as the generator type THEN the system SHALL support GET and other HTTP methods for data retrieval
4. WHEN authentication is required for HTTP sources THEN the system SHALL support referencing Kubernetes secrets for HTTP headers
5. WHEN TLS verification is needed for HTTP sources THEN the system SHALL support custom CA bundles through secret references
6. IF insecureSkipVerify is set to true for HTTP sources THEN the system SHALL skip TLS certificate verification (not recommended for production)
7. WHEN new source types are added in the future THEN the system SHALL support them through the same modular generator interface

### Requirement 2

**User Story:** As a developer extending the controller, I want a modular source generator architecture, so that I can easily add new external source types without modifying core controller logic.

#### Acceptance Criteria

1. WHEN implementing source generators THEN the system SHALL define a common interface that all source types must implement
2. WHEN adding a new source type THEN the system SHALL require only implementing the source generator interface without changing reconciliation logic
3. WHEN the controller processes different source types THEN the system SHALL use a factory pattern to instantiate the appropriate generator
4. WHEN source-specific configuration is needed THEN the system SHALL support type-specific configuration within the generator specification
5. WHEN source generators are registered THEN the system SHALL validate that each generator type has a corresponding implementation

### Requirement 3

**User Story:** As a platform engineer, I want the controller to periodically check for updates from external sources, so that my applications can automatically receive the latest configuration data.

#### Acceptance Criteria

1. WHEN an ExternalSource specifies an interval THEN the system SHALL reconcile the resource at that frequency with a minimum of 1 minute
2. WHEN checking for updates from HTTP sources THEN the system SHALL perform an HTTP HEAD request first to check ETags for optimization
3. IF the ETag matches the last handled ETag for HTTP sources THEN the system SHALL skip data fetching and requeue for the next interval
4. WHEN the ETag differs or is unavailable for HTTP sources THEN the system SHALL perform a full HTTP GET request to fetch updated data
5. WHEN spec.suspend is true THEN the system SHALL suspend all reconciliation activities for that resource

### Requirement 4

**User Story:** As a platform engineer, I want to execute command hooks before and after fetching data, so that I can prepare requests, transform responses, and validate data using whitelisted external tools.

#### Acceptance Criteria

1. WHEN pre-request hooks are specified THEN the system SHALL execute them in order before the HTTP request
2. WHEN post-request hooks are specified THEN the system SHALL execute them in order after receiving the response
3. WHEN a hook specifies a command THEN the system SHALL validate it against a whitelist before execution
4. WHEN a hook execution exceeds its timeout THEN the system SHALL terminate the process and apply the hook's retry policy
5. WHEN a hook's retry policy is "ignore" THEN the system SHALL log the failure and continue
6. WHEN a hook's retry policy is "retry" THEN the system SHALL retry the hook up to the maxRetries limit
7. WHEN a hook's retry policy is "fail" THEN the system SHALL mark the reconciliation as failed and update status conditions
8. WHEN hooks use environment variables THEN the system SHALL pass them to the command execution environment
9. IF no hooks are specified THEN the system SHALL use the raw response data as-is

### Requirement 5

**User Story:** As a platform engineer, I want fetched data to be packaged as versioned artifacts, so that I can ensure consistent and traceable deployments.

#### Acceptance Criteria

1. WHEN data is successfully fetched and transformed THEN the system SHALL package it into a .tar.gz archive
2. WHEN creating an artifact THEN the system SHALL calculate a SHA256 digest to serve as the content-based revision
3. WHEN a destinationPath is specified THEN the system SHALL place the data file at that relative path within the archive
4. WHEN uploading artifacts THEN the system SHALL store them in an S3-compatible object store
5. WHEN a new artifact is created THEN the system SHALL implement garbage collection to remove obsolete artifacts

### Requirement 6

**User Story:** As a platform engineer, I want the external data to be consumable by other Flux controllers, so that I can integrate it seamlessly into my existing GitOps workflows.

#### Acceptance Criteria

1. WHEN an ExternalSource successfully creates an artifact THEN the system SHALL create or update a child ExternalArtifact resource
2. WHEN updating the ExternalArtifact THEN the system SHALL populate its spec with the artifact URL and revision
3. WHEN the ExternalSource is deleted THEN the system SHALL clean up the associated ExternalArtifact resource
4. WHEN the ExternalArtifact is created THEN other Flux controllers SHALL be able to consume the artifact using standard Flux mechanisms
5. WHEN ownership is established THEN the ExternalArtifact SHALL be owned by the ExternalSource for proper lifecycle management

### Requirement 7

**User Story:** As a platform operator, I want comprehensive observability into the controller's operations, so that I can monitor performance and troubleshoot issues effectively.

#### Acceptance Criteria

1. WHEN reconciliation occurs THEN the system SHALL update resource status conditions with current state information
2. WHEN the controller is running THEN the system SHALL expose Prometheus metrics at a /metrics endpoint
3. WHEN reconciliation completes THEN the system SHALL record metrics for total reconciliations, duration, and success/failure status
4. WHEN making external requests THEN the system SHALL record request latency metrics by source type
5. WHEN errors occur THEN the system SHALL provide detailed error messages in status conditions and logs

### Requirement 8

**User Story:** As a platform engineer, I want the controller to handle failures gracefully, so that temporary issues don't disrupt my GitOps workflows.

#### Acceptance Criteria

1. WHEN external requests fail THEN the system SHALL retry with exponential backoff
2. WHEN the controller pod restarts THEN the system SHALL resume reconciliation based on the last known state
3. WHEN network connectivity is lost THEN the system SHALL continue retrying until connectivity is restored
4. WHEN external sources are temporarily unavailable THEN the system SHALL maintain the last successful artifact until updates are possible
5. WHEN resource validation fails THEN the system SHALL report clear error messages in the resource status

### Requirement 9

**User Story:** As a platform engineer, I want flexible artifact storage backend options, so that I can choose the appropriate storage solution for my environment and use cases.

#### Acceptance Criteria

1. WHEN configuring the controller THEN the system SHALL support multiple artifact storage backend implementations
2. WHEN using S3-compatible storage THEN the system SHALL support external object stores like MinIO, AWS S3, and compatible services
3. WHEN running in development or testing environments THEN the system SHALL support a non-persistent in-process memory store
4. WHEN switching between backends THEN the system SHALL use a modular interface that allows pluggable storage implementations
5. WHEN the in-process store is used THEN the system SHALL clearly indicate that artifacts will not persist across controller restarts

### Requirement 10

**User Story:** As a platform engineer, I want the controller to be scalable and performant, so that it can handle multiple external sources efficiently.

#### Acceptance Criteria

1. WHEN multiple ExternalSource resources exist THEN the system SHALL manage them concurrently using worker pools
2. WHEN processing hundreds of resources THEN the system SHALL maintain acceptable performance without blocking
3. WHEN reconciliation queues build up THEN the system SHALL process them efficiently using the controller-runtime framework
4. WHEN memory usage grows THEN the system SHALL implement appropriate limits and garbage collection
5. WHEN CPU usage is high THEN the system SHALL maintain responsiveness for critical operations

### Requirement 11

**User Story:** As a security engineer, I want hook command execution to be controlled by a whitelist, so that only approved commands can be executed in my cluster.

#### Acceptance Criteria

1. WHEN the controller starts THEN the system SHALL load the command whitelist from a mounted configuration file
2. WHEN a hook specifies a command THEN the system SHALL validate it against the whitelist before execution
3. WHEN a command is not in the whitelist THEN the system SHALL reject the hook execution and report an error
4. WHEN the whitelist configuration changes THEN the system SHALL support reloading without controller restart
5. WHEN hook execution occurs THEN the system SHALL run commands in an isolated sidecar container