# Release Guide

This document describes the release processes for the ExternalSource Controller, including Docker images, Helm charts, and GitHub releases.

## Overview

The project uses two distinct release workflows:

1. **Full Application Release** - Triggered by Git tags, releases Docker images, Helm charts, and creates GitHub releases
2. **Helm Chart Release** - Triggered by chart changes, releases only Helm chart updates

## Release Workflows

### 🚀 Full Application Release

**Trigger:** Git tags matching `v*` pattern (e.g., `v1.0.0`, `v2.1.3`)

**Workflow File:** `.github/workflows/release.yml`

**What gets released:**
- Multi-platform Docker images (linux/amd64, linux/arm64)
- Helm chart with matching version
- GitHub release with installation manifests
- Updated documentation

**Artifacts created:**
- `ghcr.io/oddkin/flux-externalsource-controller:v1.0.0` (Docker image)
- `oci://ghcr.io/oddkin/charts/flux-externalsource-controller:1.0.0` (Helm chart)
- GitHub release with `install.yaml` and `flux-externalsource-controller-1.0.0.tgz`
- Checksums file for verification

### ⚓ Helm Chart Release

**Trigger:** 
- Changes to `charts/` directory pushed to `main` branch
- Manual workflow dispatch

**Workflow File:** `.github/workflows/helm-release.yml`

**What gets released:**
- Helm chart with auto-incremented patch version
- OCI registry push
- Chart repository index update

**Artifacts created:**
- `oci://ghcr.io/oddkin/charts/flux-externalsource-controller:0.1.1` (Helm chart only)

## How to Release

### 🎯 Application Release (Recommended for most releases)

Use this for new features, bug fixes, or any code changes.

#### 1. Prepare the Release

```bash
# Ensure you're on the main branch and up to date
git checkout main
git pull origin main

# Run tests to ensure everything works
make test
make test-e2e  # If Docker is available

# Update CHANGELOG.md (if you maintain one)
vim CHANGELOG.md
```

#### 2. Create and Push the Tag

```bash
# Create a semantic version tag
git tag v1.0.0

# Push the tag to trigger the release
git push origin v1.0.0
```

#### 3. Monitor the Release

1. Go to **GitHub Actions** tab
2. Watch the "Release" workflow progress
3. Check that all jobs complete successfully:
   - `build-and-push-image`
   - `build-and-push-helm-chart`
   - `create-release-artifacts`
   - `update-documentation`

#### 4. Verify the Release

```bash
# Check Docker image
docker pull ghcr.io/oddkin/flux-externalsource-controller:v1.0.0

# Check Helm chart
helm pull oci://ghcr.io/oddkin/charts/flux-externalsource-controller --version 1.0.0

# Verify GitHub release exists
gh release view v1.0.0
```

### 📦 Chart-Only Release

Use this for Helm chart improvements that don't require code changes.

#### Method 1: Automatic (Recommended)

```bash
# Make changes to chart files
vim charts/flux-externalsource-controller/values.yaml
vim charts/flux-externalsource-controller/templates/deployment.yaml

# Commit and push changes
git add charts/
git commit -m "feat: add resource limits configuration option"
git push origin main
```

The workflow will automatically:
- Detect changes in `charts/` directory
- Increment patch version (e.g., `0.1.0` → `0.1.1`)
- Package and release the chart

#### Method 2: Manual Trigger

**Via GitHub UI:**
1. Go to **GitHub Actions** tab
2. Select "Helm Chart Release" workflow
3. Click "Run workflow"
4. Select `main` branch and click "Run workflow"

**Via GitHub CLI:**
```bash
gh workflow run helm-release.yml
```

## Version Management

### Semantic Versioning

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (`v2.0.0`): Breaking changes
- **MINOR** (`v1.1.0`): New features, backward compatible
- **PATCH** (`v1.0.1`): Bug fixes, backward compatible

### Version Strategies

| Change Type | Recommended Approach | Example |
|-------------|---------------------|---------|
| Bug fixes | Application release with patch version | `v1.0.0` → `v1.0.1` |
| New features | Application release with minor version | `v1.0.0` → `v1.1.0` |
| Breaking changes | Application release with major version | `v1.0.0` → `v2.0.0` |
| Chart improvements | Chart-only release (auto-increment) | Chart `0.1.0` → `0.1.1` |
| Chart major changes | Application release with manual version | Coordinate with app version |

### Chart Versioning

- **Application releases**: Chart version matches Git tag version
- **Chart-only releases**: Auto-incremented patch version
- **Manual override**: Edit `charts/flux-externalsource-controller/Chart.yaml` before tagging

## Pre-release Versions

### Alpha/Beta/RC Releases

```bash
# Create pre-release tags
git tag v1.0.0-alpha.1
git tag v1.0.0-beta.1
git tag v1.0.0-rc.1

# Push to trigger release
git push origin v1.0.0-alpha.1
```

**Behavior:**
- Creates Docker images with pre-release tags
- Marks GitHub release as "pre-release"
- Skips documentation updates
- Useful for testing before stable release

### Development Builds

The CI workflow creates development images on every push to `main`:

- `ghcr.io/oddkin/flux-externalsource-controller:main`
- `ghcr.io/oddkin/flux-externalsource-controller:main-<sha>`

## Release Artifacts

### Docker Images

**Registry:** `ghcr.io/oddkin/flux-externalsource-controller`

**Tags created:**
- `v1.0.0` (exact version)
- `1.0.0` (without 'v' prefix)
- `1.0` (major.minor)
- `1` (major only)

**Platforms:**
- `linux/amd64`
- `linux/arm64`

### Helm Charts

**Registry:** `oci://ghcr.io/oddkin/charts/flux-externalsource-controller`

**Installation:**
```bash
# Install specific version
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller --version 1.0.0

# Install latest
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller
```

### GitHub Releases

**Artifacts included:**
- `install.yaml` - Complete installation manifest
- `flux-externalsource-controller-1.0.0.tgz` - Helm chart archive
- `checksums.txt` - SHA256 checksums for verification

**Installation from GitHub:**
```bash
# Direct installation
kubectl apply -f https://github.com/oddkinco/flux-externalsource-controller/releases/download/v1.0.0/install.yaml

# Download and verify
curl -LO https://github.com/oddkinco/flux-externalsource-controller/releases/download/v1.0.0/flux-externalsource-controller-1.0.0.tgz
curl -LO https://github.com/oddkinco/flux-externalsource-controller/releases/download/v1.0.0/checksums.txt
sha256sum -c checksums.txt
```

## Troubleshooting

### Common Issues

#### 1. Release Workflow Fails

**Docker build fails:**
```bash
# Test locally
make docker-build IMG=test:latest
```

**Helm packaging fails:**
```bash
# Validate chart
helm lint charts/flux-externalsource-controller
helm template flux-externalsource-controller charts/flux-externalsource-controller
```

#### 2. Tag Already Exists

```bash
# Delete local tag
git tag -d v1.0.0

# Delete remote tag
git push origin :refs/tags/v1.0.0

# Create new tag
git tag v1.0.0
git push origin v1.0.0
```

#### 3. Chart Version Conflicts

```bash
# Check current chart version
grep '^version:' charts/flux-externalsource-controller/Chart.yaml

# Manually update if needed
sed -i 's/version: 0.1.0/version: 1.0.0/' charts/flux-externalsource-controller/Chart.yaml
```

### Debugging Workflows

#### View workflow logs:
```bash
# List recent workflow runs
gh run list --workflow=release.yml

# View specific run
gh run view <run-id>

# Download logs
gh run download <run-id>
```

#### Re-run failed workflows:
```bash
# Re-run failed jobs
gh run rerun <run-id> --failed

# Re-run entire workflow
gh run rerun <run-id>
```

## Security Considerations

### Registry Access

- Docker images and Helm charts are pushed to GitHub Container Registry (ghcr.io)
- Uses `GITHUB_TOKEN` with appropriate permissions
- Images are public by default (can be configured as private)

### Supply Chain Security

- Multi-platform builds with attestations
- SHA256 checksums for all artifacts
- Signed commits recommended for releases
- Dependency scanning in CI pipeline

### Secrets Management

- No secrets required for public releases
- `GITHUB_TOKEN` automatically provided by GitHub Actions
- Additional secrets can be configured for private registries

## Best Practices

### Before Releasing

1. **Test thoroughly:**
   ```bash
   make test
   make test-e2e
   make lint
   ```

2. **Update documentation:**
   - README.md
   - CHANGELOG.md
   - API documentation

3. **Review changes:**
   ```bash
   git log --oneline $(git describe --tags --abbrev=0)..HEAD
   ```

### Release Checklist

- [ ] All tests pass
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if maintained)
- [ ] Version number follows semantic versioning
- [ ] No breaking changes in patch releases
- [ ] Pre-release testing completed (for major releases)

### Post-Release

1. **Verify artifacts:**
   - Test Docker image installation
   - Test Helm chart installation
   - Verify GitHub release assets

2. **Update dependent projects:**
   - Update version references in other repositories
   - Notify users of new release

3. **Monitor for issues:**
   - Watch for bug reports
   - Monitor download metrics
   - Check for security vulnerabilities

## Automation

### Automated Testing

The release workflows include:
- Unit tests
- Integration tests
- Security scanning
- Lint checks
- Chart validation

### Automated Documentation

- Version references in README.md are automatically updated
- Chart documentation is updated with new versions
- Release notes are auto-generated from commits

### Notifications

Configure notifications for:
- Failed releases
- Security vulnerabilities
- New release announcements

## Examples

### Example Release Commands

```bash
# Patch release (bug fix)
git tag v1.0.1
git push origin v1.0.1

# Minor release (new feature)
git tag v1.1.0
git push origin v1.1.0

# Major release (breaking change)
git tag v2.0.0
git push origin v2.0.0

# Pre-release
git tag v1.1.0-beta.1
git push origin v1.1.0-beta.1
```

### Example Chart Updates

```bash
# Add new configuration option
vim charts/flux-externalsource-controller/values.yaml
vim charts/flux-externalsource-controller/templates/deployment.yaml
git add charts/
git commit -m "feat: add nodeSelector configuration"
git push origin main
# → Triggers chart release with auto-incremented version
```

This release guide ensures consistent, reliable releases while providing flexibility for different types of updates. Follow these processes to maintain high-quality releases and clear version management.