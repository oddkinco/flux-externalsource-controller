# Implementation Plan

- [x] 1. Prepare for refactoring and backup current state
  - Verify current build passes with `make build` and `make test`
  - Create git branch for refactoring work
  - Document current state for rollback reference
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 2. Update directory structure and file names
  - [x] 2.1 Rename Helm chart directory from `charts/fx-controller/` to `charts/flux-external-controller/`
    - Move entire directory structure to new name
    - _Requirements: 2.1_

- [x] 3. Update Helm chart configuration files
  - [x] 3.1 Update Chart.yaml with new naming convention
    - Change chart name from "fx-controller" to "flux-external-controller"
    - Update home and sources URLs to use new repository path
    - Update maintainer name reference
    - _Requirements: 2.1, 2.2, 4.1, 4.2_
  
  - [x] 3.2 Update Helm chart README.md
    - Replace all "fx-controller" references with "flux-external-controller"
    - Update GitHub repository URLs to new path
    - _Requirements: 6.1, 6.2, 6.3_
  
  - [x] 3.3 Update Helm template files
    - Replace all "fx-controller" references in template files
    - Update labels and names to use new convention
    - _Requirements: 2.3, 3.2_

- [x] 4. Update Go API group definitions
  - [x] 4.1 Update groupversion_info.go API group
    - Change API group from "source.example.com" to "source.flux.oddkin.co"
    - Update GroupVersion variable and kubebuilder markers
    - _Requirements: 5.1, 5.2_

- [x] 5. Update PROJECT file configuration
  - [x] 5.1 Update PROJECT file with new naming
    - Change projectName from "fx-controller" to "flux-external-controller"
    - Change domain from "example.com" to "flux.oddkin.co"
    - Verify repo field matches new module path
    - _Requirements: 1.1, 4.1_

- [x] 6. Update documentation files
  - [x] 6.1 Update main README.md
    - Replace all "fx-controller" references with "flux-external-controller"
    - Update GitHub repository URLs from "github.com/example/fx-controller" to "github.com/oddkin/flux-externalsource-controller"
    - Update API group references in examples from "source.example.com" to "source.flux.oddkin.co"
    - _Requirements: 4.1, 4.2, 5.4, 6.1, 6.2, 6.3_
  
  - [x] 6.2 Update documentation in docs/ directory
    - Update all references to "fx-controller" in development.md, extending-generators.md, and releases.md
    - Update GitHub repository URLs and API group references
    - _Requirements: 4.1, 4.2, 5.4, 6.3_

- [x] 7. Update example files
  - [x] 7.1 Update all example YAML files
    - Change apiVersion from "source.example.com/v1alpha1" to "source.flux.oddkin.co/v1alpha1"
    - Update labels from "app.kubernetes.io/name: fx-controller" to "app.kubernetes.io/name: flux-externalsource-controller"
    - _Requirements: 5.3, 5.4, 6.2_

- [x] 8. Update configuration and sample files
  - [x] 8.1 Update sample resource files
    - Change apiVersion in config/samples/ files from "source.example.com/v1alpha1" to "source.flux.oddkin.co/v1alpha1"
    - _Requirements: 5.4_
  
  - [x] 8.2 Update RBAC configuration files
    - Update API group references in all RBAC files from "source.example.com" to "source.flux.oddkin.co"
    - Update comments referencing the old API group
    - _Requirements: 5.1, 5.2_

- [x] 9. Update build and CI/CD configurations
  - [x] 9.1 Update GitHub workflow files
    - Replace "fx-controller" references in .github/workflows/ files
    - Update Helm chart path references
    - _Requirements: 3.1, 3.2_
  
  - [x] 9.2 Update Makefile references
    - Replace any "fx-controller" references in build targets and variables
    - _Requirements: 3.1_

- [x] 10. Update existing spec files
  - [x] 10.1 Update externalsource-controller spec files
    - Replace "fx-controller" references in existing spec documentation
    - Update API group references where applicable
    - _Requirements: 6.3, 6.4_

- [x] 11. Regenerate auto-generated files
  - [x] 11.1 Regenerate CRD manifests
    - Run `make manifests` to regenerate CRD files with new API group
    - Verify new CRD files use "source.flux.oddkin.co" API group
    - _Requirements: 5.1, 5.3_
  
  - [x] 11.2 Regenerate DeepCopy methods
    - Run `make generate` to regenerate zz_generated.deepcopy.go
    - _Requirements: 5.2_

- [x] 12. Validate and test changes
  - [x] 12.1 Run build validation
    - Execute `make build` to verify Go code compiles
    - Execute `make test` to verify unit tests pass
    - _Requirements: 1.1, 1.2, 1.3, 1.4_
  
  - [x] 12.2 Validate Helm chart
    - Run `helm lint charts/flux-external-controller` to verify chart syntax
    - Run `helm template` to verify template rendering
    - _Requirements: 2.1, 2.2, 2.3_
  
  - [x] 12.3 Search for references to fx-controller in tests
    - Run grep searches for "fx-controller" in tests and correct
  - [ ] 12.4 Run comprehensive test suite
    - Execute any integration tests if available
    - Verify e2e tests pass with new naming
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 13. Final verification and cleanup
  - [x] 13.1 Search for any remaining old references
    - Run grep searches to verify no "fx-controller", "source.example.com", or "github.com/example" references remain
    - _Requirements: 1.3, 4.2, 5.4_
  
  - [x] 13.2 Manual review of critical files
    - Review generated CRD files for correct API group
    - Verify Helm chart metadata is correct
    - Check example files use correct apiVersion
    - _Requirements: 2.1, 2.2, 5.1, 5.3_