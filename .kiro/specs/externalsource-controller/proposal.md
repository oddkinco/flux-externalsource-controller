## **ExternalSource Controller: Design Proposal**

### 

### **Executive Summary**

This document proposes the design and implementation of the **ExternalSource Controller** for Flux, a Kubernetes operator built using the Kubebuilder framework. The controller's primary function is to integrate external, non-Git data sources (initially via HTTP APIs) into the GitOps workflow powered by Flux. It achieves this by periodically fetching data from a specified endpoint, optionally transforming it, packaging it as a versioned artifact, and making it available for consumption by other Flux controllers (e.g., Kustomization, HelmRelease) via an ExternalArtifact custom resource. This architecture provides a reliable, scalable, and observable solution for managing dynamic configuration data within Kubernetes.

### **Project Goals**

1. **CRD Implementation:** Establish a robust ExternalSource Custom Resource Definition (CRD) as the primary user-facing API.  
2. **Asynchronous Reconciliation:** Implement a stateful, asynchronous reconciliation loop to reliably manage the lifecycle of external resources.  
3. **Efficient Data Fetching:** Integrate with external HTTP APIs, leveraging mechanisms like ETags to minimize redundant data processing and artifact generation.  
4. **Secure and Reliable Artifact Management:** Package fetched data into content-addressable .tar.gz archives, store them in a durable artifact backend, and implement garbage collection for obsolete artifacts.  
5. **Seamless Flux Interoperability:** Natively integrate with the Flux toolkit by managing the lifecycle of a child ExternalArtifact resource, making the external data easily consumable.  
6. **Actionable Observability:** Provide detailed metrics and status conditions to ensure operators have clear insight into the controller's performance and health.

### **I. Architecture Overview (Kubebuilder Operator)**

The controller will be implemented as a dedicated Kubernetes operator using the **Kubebuilder** framework. This approach provides significant advantages over a stateless webhook model:

* **Asynchronous & Stateful Reconciliation:** The operator maintains an active reconciliation loop for each ExternalSource resource. This allows for long-running operations, intelligent retries with exponential backoff, and robust state management through the resource's status subresource.  
* **Scalability & Performance:** The underlying controller-runtime library manages work queues and worker pools, enabling the controller to efficiently manage hundreds or thousands of ExternalSource resources concurrently without blocking.  
* **Robustness:** The operator pattern is resilient to failures. If the controller pod restarts, it will resume reconciliation based on the last known state of the custom resources in the cluster.

### **II. Custom Resource Definition (CRD)**

The controller introduces the ExternalSource CRD, which will be the primary interface for users.

#### **Kind: ExternalSource (source.example.com/v1alpha1)**

**spec**

| Field | Type | Required | Description |
| :---- | :---- | :---- | :---- |
| interval | metav1.Duration | Yes | The frequency at which to check for updates. Minimum value 1m. |
| suspend | bool | No | If true, suspends reconciliations for this source. |
| destinationPath | string | No | Relative path within the artifact bundle to place the data file (e.g., config/values.yaml). |
| transform | object | No | A definition for transforming the raw HTTP response. See transform spec below. |
| generator | object | Yes | Defines the method for acquiring the source data. Initially, only http is supported. |

**generator.http Spec**

| Field | Type | Required | Description |
| :---- | :---- | :---- | :---- |
| url | string | Yes | The target external HTTP API endpoint. |
| method | string | No | HTTP method (defaults to GET). |
| headersSecretRef | v1.LocalObjectReference | No | Reference to a Kubernetes Secret containing headers (e.g., Authorization). |
| caBundleSecretRef | v1.LocalObjectReference | No | Reference to a Secret key containing a PEM-encoded CA bundle for TLS verification. |
| insecureSkipVerify | bool | No | **(Not Recommended)** Allows skipping TLS certificate verification. Ignored if caBundleSecretRef is set. |

**transform Spec**

| Field | Type | Required | Description |
| :---- | :---- | :---- | :---- |
| type | string | Yes | The transformation language to use. Initial support for cel. |
| expression | string | Yes | The CEL expression to apply to the raw response body. The result becomes the artifact content. |

**status Subresource**

| Field | Type | Description |
| :---- | :---- | :---- |
| conditions | \[\]metav1.Condition | Standard Kubernetes conditions (Ready, Fetching, etc.) reflecting the resource's state. |
| artifact | object | The last successfully generated artifact, containing its URL, revision (SHA256 digest), and metadata. |
| lastHandledETag | string | The HTTP ETag from the last successful fetch, used for conditional reconciliation. |
| observedGeneration | int64 | The last reconciled metadata.generation of the resource. |

### **III. Reconciliation Logic**

The controller's reconciliation loop will execute the following stateful, asynchronous process:

1. **Read State:** Fetch the ExternalSource resource. If the resource is being deleted or spec.suspend is true, perform cleanup and cease further action.  
2. **Conditional Check (Optimization):**  
   * Perform an HTTP HEAD request to the target url.  
   * Compare the ETag header from the response with the status.lastHandledETag.  
   * If the ETags match, the remote content has not changed. The controller will update the Ready condition and requeue the resource for its next check at spec.interval, skipping the rest of the steps.  
3. **Fetch Data:**  
   * If credentials are required, fetch the referenced headersSecretRef.  
   * Execute the full HTTP GET request, respecting TLS settings (caBundleSecretRef or insecureSkipVerify).  
   * On failure, update the status conditions with an error, and requeue the request with exponential backoff.  
4. **Transform Data (Sandboxed):**  
   * If spec.transform is defined, execute the transformation logic.  
   * The execution will be sandboxed with strict timeouts and memory limits to prevent malicious or poorly written expressions from impacting the controller.  
5. **Package Artifact:**  
   * Create a file with the final (transformed or raw) data at the specified destinationPath.  
   * Package the file into a .tar.gz archive.  
   * Calculate the SHA256 digest of the archive to serve as the new content-based revision (e.g., sha256:\<digest\>).  
6. **Upload & Manage Artifacts:**  
   * Upload the new archive to the artifact storage backend (e.g., MinIO/S3).  
   * **Garbage Collection:** List all artifacts associated with this ExternalSource and delete any that do not match the newly uploaded revision, ensuring only the latest artifact is retained.  
7. **Sync Child ExternalArtifact:**  
   * Create or update the child ExternalArtifact resource owned by the ExternalSource.  
   * Populate its spec with the URL and revision of the new artifact.  
8. **Update Status:**  
   * Update the ExternalSource.status with the new artifact details, the lastHandledETag, and a Ready condition set to True.

### **IV. Observability & Monitoring**

To provide operational insight, the controller will expose a /metrics endpoint with Prometheus metrics, including:

* externalsource\_reconciliation\_total{kind, name, namespace, status}: A counter for total reconciliations (success/failure).  
* externalsource\_reconciliation\_duration\_seconds{kind, name, namespace}: A histogram of reconciliation latency.  
* externalsource\_api\_request\_latency\_seconds{host}: A histogram of latency for external HTTP API calls.  
* gotk\_reconcile\_condition: The standard Flux condition metric, indicating the health of each ExternalSource resource.

### **V. Requirements & Dependencies**

| Requirement | Category | Detail |
| :---- | :---- | :---- |
| **Kubernetes Cluster** | Environment | Version 1.25+ recommended. |
| **FluxCD** | Environment | The Flux source-controller must be installed to provide the ExternalArtifact CRD and an artifact storage backend. |
| **Controller Manager** | Implementation | The operator will run as a standard Kubernetes Deployment. |
| **Go Toolchain** | Development | Required for building the controller from source. |
| **Artifact Storage** | Environment | An S3-compatible object store (e.g., MinIO, AWS S3) accessible by the controller. |
| **Networking** | Environment | The controller Pod requires network egress to the external HTTP APIs defined in ExternalSource resources. |

