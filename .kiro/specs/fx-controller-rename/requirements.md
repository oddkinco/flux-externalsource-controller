# Requirements Document

## Introduction

This feature involves refactoring all references to "fx-controller" throughout the codebase to use the more descriptive name "flux-externalsource-controller". This includes updating file names, directory names, configuration files, documentation, and any code references to ensure consistency and clarity in the project naming.

## Requirements

### Requirement 1

**User Story:** As a developer working on the project, I want all references to "fx-controller" to be renamed to "flux-externalsource-controller" so that the project has a clear and consistent naming convention that better describes its purpose.

#### Acceptance Criteria

1. WHEN examining the project structure THEN all directory names containing "fx-controller" SHALL be renamed to "flux-externalsource-controller"
2. WHEN examining file names THEN all files containing "fx-controller" in their name SHALL be renamed to use "flux-externalsource-controller"
3. WHEN examining file contents THEN all string references to "fx-controller" SHALL be updated to "flux-externalsource-controller"
4. WHEN examining configuration files THEN all configuration values referencing "fx-controller" SHALL be updated to "flux-externalsource-controller"

### Requirement 2

**User Story:** As a developer deploying the application, I want all Helm chart references to use the new naming convention so that deployments use consistent naming.

#### Acceptance Criteria

1. WHEN examining the Helm chart directory THEN the chart directory name SHALL be "flux-externalsource-controller"
2. WHEN examining Chart.yaml THEN the chart name SHALL be "flux-externalsource-controller"
3. WHEN examining Helm templates THEN all references to "fx-controller" SHALL be updated to "flux-externalsource-controller"
4. WHEN examining values.yaml THEN all configuration keys and values SHALL use "flux-externalsource-controller"

### Requirement 3

**User Story:** As a developer building and deploying the application, I want all build and deployment configurations to use the new naming convention so that artifacts and deployments are consistently named.

#### Acceptance Criteria

1. WHEN examining the Makefile THEN all targets and variables referencing "fx-controller" SHALL use "flux-externalsource-controller"
2. WHEN examining Kubernetes manifests THEN all resource names and labels SHALL use "flux-externalsource-controller"
3. WHEN examining Docker configurations THEN image names and tags SHALL use "flux-externalsource-controller"
4. WHEN examining Kustomize configurations THEN all resource references SHALL use "flux-externalsource-controller"

### Requirement 4

**User Story:** As a developer working with Go modules, I want the module path to be changed from any "github.com/example/fx-controller" references to "github.com/oddkin/flux-externalsource-controller" so that the module path reflects the correct repository and naming.

#### Acceptance Criteria

1. WHEN examining go.mod THEN the module path SHALL be "github.com/oddkin/flux-externalsource-controller"
2. WHEN examining Go import statements THEN all imports SHALL use the new module path
3. WHEN examining generated code THEN all module references SHALL use the new path
4. WHEN examining build configurations THEN all module path references SHALL be updated

### Requirement 5

**User Story:** As a developer working with the Kubernetes API, I want the API group domain to be changed from "source.example.com" to "source.flux.oddkin.co" so that the API uses a proper domain that reflects the project ownership.

#### Acceptance Criteria

1. WHEN examining CRD definitions THEN the API group SHALL be "source.flux.oddkin.co"
2. WHEN examining Go API types THEN all group references SHALL use "source.flux.oddkin.co"
3. WHEN examining generated manifests THEN all API versions SHALL use "source.flux.oddkin.co"
4. WHEN examining example resources THEN all apiVersion fields SHALL use "source.flux.oddkin.co"

### Requirement 6

**User Story:** As a developer reading documentation and examples, I want all documentation to reflect the new naming convention so that instructions and examples are accurate and consistent.

#### Acceptance Criteria

1. WHEN examining README files THEN all references to "fx-controller" SHALL be updated to "flux-externalsource-controller"
2. WHEN examining example files THEN all configuration examples SHALL use "flux-externalsource-controller"
3. WHEN examining documentation files THEN all references SHALL use the new naming convention
4. WHEN examining code comments THEN all comments referencing "fx-controller" SHALL be updated