# Product Overview

The ExternalSource Controller is a Kubernetes operator that integrates external, non-Git data sources into GitOps workflows powered by Flux. It enables fetching, transforming, and packaging external HTTP-based data as versioned artifacts consumable by other Flux controllers.

## Key Features

- **Modular Source Generators**: Pluggable architecture supporting HTTP sources with extensibility for future source types
- **Data Transformation**: Optional CEL-based transformation of fetched data  
- **Artifact Management**: Automatic packaging and versioning of external data as .tar.gz archives
- **Flux Integration**: Seamless integration with existing Flux controllers through ExternalArtifact resources
- **Observability**: Comprehensive metrics and status reporting for monitoring and troubleshooting

## Core Resources

- **ExternalSource**: Defines external data sources to fetch and reconcile
- **ExternalArtifact**: Represents packaged artifacts created from external sources

The controller implements an asynchronous reconciliation pattern following Kubernetes operator best practices.