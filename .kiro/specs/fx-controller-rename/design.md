# Design Document

## Overview

This design outlines the comprehensive refactoring approach to rename "fx-controller" to "flux-external-controller", update the API group from "source.example.com" to "source.flux.oddkin.co", and change the Go module path from "github.com/example/fx-controller" to "github.com/oddkin/flux-externalsource-controller". The refactoring will be executed systematically to ensure all references are updated consistently across the entire codebase.

## Architecture

The refactoring will follow a structured approach organized into logical groups:

1. **File and Directory Renaming**: Physical file and directory structure changes
2. **Content Updates**: String replacements within files
3. **Generated Code Regeneration**: Updating auto-generated files
4. **Validation**: Ensuring all changes are consistent and functional

## Components and Interfaces

### File System Changes

**Directory Renaming:**
- `charts/fx-controller/` → `charts/flux-external-controller/`

**File Renaming:**
- All CRD files with `source.example.com` prefix will be regenerated with new names
- No other files require renaming based on current analysis

### Content Update Categories

**1. Helm Chart Updates**
- Chart.yaml: name, home, sources, maintainer references
- Template files: all fx-controller references
- Values.yaml: configuration keys and default values
- README.md: documentation and examples

**2. Kubernetes Manifests**
- CRD definitions: API group changes
- RBAC files: API group and resource references
- Sample resources: apiVersion fields
- Configuration files: labels and names

**3. Go Code Updates**
- API group definitions in groupversion_info.go
- Generated CRD files (will be regenerated)
- Import statements (if any exist)

**4. Documentation Updates**
- README.md: installation instructions, examples, repository URLs
- docs/ directory: all references to old names
- examples/ directory: apiVersion fields and labels

**5. Build and CI/CD Updates**
- Makefile: image names and targets
- GitHub workflows: chart references and build configurations
- PROJECT file: project name updates

## Data Models

### Replacement Mappings

The following string replacements will be applied systematically:

```
fx-controller → flux-external-controller
source.example.com → source.flux.oddkin.co
github.com/example/fx-controller → github.com/oddkin/flux-externalsource-controller
```

### File Categories

**Category 1: Direct String Replacement**
- Documentation files (*.md)
- YAML configuration files
- Helm chart files
- Example files

**Category 2: API Group Updates**
- Go API type definitions
- CRD base files
- RBAC configuration files
- Sample resource files

**Category 3: Generated Files (Regeneration Required)**
- CRD manifest files
- DeepCopy generated code
- Any kubebuilder-generated files

## Error Handling

### Validation Strategy

1. **Pre-change Validation**
   - Verify current build and test suite passes
   - Document current state for rollback reference

2. **Post-change Validation**
   - Run code generation to update generated files
   - Execute full test suite
   - Verify Helm chart linting passes
   - Validate Kubernetes manifest syntax

3. **Rollback Strategy**
   - Maintain backup of original files
   - Use git for version control and easy rollback
   - Test rollback procedure before implementation

### Risk Mitigation

- **Broken References**: Systematic search and replace to catch all instances
- **Generated Code Conflicts**: Regenerate all auto-generated files after changes
- **Build Failures**: Validate build process after each major category of changes
- **Deployment Issues**: Test Helm chart installation in development environment

## Testing Strategy

### Validation Steps

1. **Static Analysis**
   - Grep searches to verify all old references are updated
   - Lint checks for Helm charts and YAML files
   - Go build verification

2. **Functional Testing**
   - Run existing unit test suite
   - Execute integration tests if available
   - Validate Helm chart installation

3. **Manual Verification**
   - Review generated CRD files for correct API groups
   - Verify example resources use correct apiVersion
   - Check documentation for consistency

### Test Sequence

1. Update file and directory names
2. Update content with string replacements
3. Regenerate auto-generated code
4. Run validation tests
5. Manual review of critical files
6. Full build and test execution

## Implementation Considerations

### Order of Operations

The refactoring must follow a specific sequence to avoid breaking dependencies:

1. **Preparation**: Backup current state and verify clean build
2. **Directory Renaming**: Update physical file structure
3. **Content Updates**: Apply string replacements systematically
4. **Code Generation**: Regenerate all auto-generated files
5. **Validation**: Run tests and manual verification
6. **Documentation**: Update any remaining documentation references

### Critical Files

Special attention required for:
- `api/v1alpha1/groupversion_info.go`: Core API group definition
- `PROJECT`: Kubebuilder project configuration
- `charts/fx-controller/Chart.yaml`: Helm chart metadata
- All CRD files: Will be regenerated with new API group
- GitHub workflows: Build and release automation

### Dependencies

- Kubebuilder for code regeneration
- Helm for chart validation
- Go toolchain for build verification
- Git for version control and rollback capability