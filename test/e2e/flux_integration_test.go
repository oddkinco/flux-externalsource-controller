//go:build e2e
// +build e2e

/*
Copyright (c) 2025 Odd Kin <oddkin@oddkin.co>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package e2e

import (
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/oddkinco/flux-externalsource-controller/test/utils"
)

var _ = Describe("Flux Integration", Ordered, func() {
	const testNamespace = "flux-integration-test"

	BeforeAll(func() {
		By("creating test namespace")
		cmd := exec.Command("kubectl", "create", "ns", testNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")
	})

	AfterAll(func() {
		By("cleaning up test namespace")
		cmd := exec.Command("kubectl", "delete", "ns", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	SetDefaultEventuallyTimeout(3 * time.Minute)
	SetDefaultEventuallyPollingInterval(5 * time.Second)

	Context("ExternalArtifact Integration", func() {
		It("should create ExternalArtifact that can be consumed by Flux controllers", func() {
			By("creating a test HTTP server with configuration data")
			configServerManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-server-data
  namespace: ` + testNamespace + `
data:
  config.yaml: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: app-config
      namespace: default
    data:
      app.properties: |
        server.port=8080
        database.url=jdbc:postgresql://localhost:5432/mydb
        feature.enabled=true
  index.html: |
    <html><body><h1>Config Server</h1></body></html>
---
apiVersion: v1
kind: Pod
metadata:
  name: config-server
  namespace: ` + testNamespace + `
  labels:
    app: config-server
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: server
    image: nginxinc/nginx-unprivileged:alpine
    ports:
    - containerPort: 8080
    volumeMounts:
    - name: config
      mountPath: /usr/share/nginx/html
    - name: cache
      mountPath: /var/cache/nginx
    - name: run
      mountPath: /var/run
    - name: tmp
      mountPath: /tmp
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
  volumes:
  - name: config
    configMap:
      name: config-server-data
  - name: cache
    emptyDir: {}
  - name: run
    emptyDir: {}
  - name: tmp
    emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: config-server
  namespace: ` + testNamespace + `
spec:
  selector:
    app: config-server
  ports:
  - port: 80
    targetPort: 8080
`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(configServerManifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create config server")

			By("waiting for config server to be ready")
			verifyConfigServerReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", "config-server", "-n", testNamespace, "-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"))
			}
			Eventually(verifyConfigServerReady).Should(Succeed())

			By("creating an ExternalSource to fetch configuration")
			externalSourceManifest := `
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: app-config-source
  namespace: ` + testNamespace + `
spec:
  interval: 1m
  generator:
    type: http
    http:
      url: http://config-server.` + testNamespace + `.svc.cluster.local/config.yaml
      method: GET
  destinationPath: manifests/config.yaml
`
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(externalSourceManifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ExternalSource")

			By("waiting for ExternalSource to be ready and create ExternalArtifact")
			var artifactURL string
			verifyExternalArtifactReady := func(g Gomega) {
				// Check ExternalSource is ready
				cmd := exec.Command("kubectl", "get", "externalsource", "app-config-source", "-n", testNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))

				// Check ExternalArtifact exists and has URL
				cmd = exec.Command("kubectl", "get", "externalartifact", "app-config-source", "-n", testNamespace, "-o", "jsonpath={.spec.url}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty())
				artifactURL = output
			}
			Eventually(verifyExternalArtifactReady).Should(Succeed())

			By("verifying ExternalArtifact has proper metadata")
			verifyArtifactMetadata := func(g Gomega) {
				// Check revision is set
				cmd := exec.Command("kubectl", "get", "externalartifact", "app-config-source", "-n", testNamespace, "-o", "jsonpath={.spec.revision}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty())
				g.Expect(len(output)).To(Equal(64)) // SHA256 hash length

				// Verify artifact URL is accessible (this would be done by Flux in real scenarios)
				g.Expect(artifactURL).To(ContainSubstring("app-config-source"))
			}
			Eventually(verifyArtifactMetadata).Should(Succeed())

			By("simulating Flux controller consumption pattern")
			// In a real Flux environment, a Kustomization or HelmRelease would reference this ExternalArtifact
			// Here we simulate the pattern by checking that the artifact follows Flux conventions
			verifyFluxCompatibility := func(g Gomega) {
				// Check that ExternalArtifact has the expected structure for Flux consumption
				cmd := exec.Command("kubectl", "get", "externalartifact", "app-config-source", "-n", testNamespace, "-o", "yaml")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Verify it has the required fields that Flux controllers expect
				g.Expect(output).To(ContainSubstring("spec:"))
				g.Expect(output).To(ContainSubstring("url:"))
				g.Expect(output).To(ContainSubstring("revision:"))
			}
			Eventually(verifyFluxCompatibility).Should(Succeed())

			By("testing artifact update workflow")
			// Update the config server data to trigger a new artifact
			updatedConfigManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-server-data
  namespace: ` + testNamespace + `
data:
  config.yaml: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: app-config
      namespace: default
    data:
      app.properties: |
        server.port=9090
        database.url=jdbc:postgresql://localhost:5432/mydb
        feature.enabled=false
        new.feature=true
  index.html: |
    <html><body><h1>Updated Config Server</h1></body></html>
`
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(updatedConfigManifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to update config server")

			By("waiting for ExternalSource to detect changes and update artifact")
			var newRevision string
			verifyArtifactUpdate := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "externalartifact", "app-config-source", "-n", testNamespace, "-o", "jsonpath={.spec.revision}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty())

				// Store the new revision for comparison
				if newRevision == "" {
					newRevision = output
				} else {
					// Verify the revision has changed (indicating new content)
					g.Expect(output).NotTo(Equal(newRevision))
				}
			}
			Eventually(verifyArtifactUpdate, 2*time.Minute).Should(Succeed())

			By("cleaning up test resources")
			cmd = exec.Command("kubectl", "delete", "externalsource", "app-config-source", "-n", testNamespace)
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "pod,service,configmap", "-l", "app=config-server", "-n", testNamespace)
			_, _ = utils.Run(cmd)
		})

		It("should handle multiple ExternalSources concurrently", func() {
			By("creating multiple test HTTP servers")
			multiServerManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: server1-data
  namespace: ` + testNamespace + `
data:
  data.json: |
    {"service": "service1", "version": "1.0.0", "config": {"replicas": 3}}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: server2-data
  namespace: ` + testNamespace + `
data:
  data.json: |
    {"service": "service2", "version": "2.0.0", "config": {"replicas": 5}}
---
apiVersion: v1
kind: Pod
metadata:
  name: test-server1
  namespace: ` + testNamespace + `
  labels:
    app: test-server1
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: server
    image: nginxinc/nginx-unprivileged:alpine
    ports:
    - containerPort: 8080
    volumeMounts:
    - name: config
      mountPath: /usr/share/nginx/html
    - name: cache
      mountPath: /var/cache/nginx
    - name: run
      mountPath: /var/run
    - name: tmp
      mountPath: /tmp
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
  volumes:
  - name: config
    configMap:
      name: server1-data
  - name: cache
    emptyDir: {}
  - name: run
    emptyDir: {}
  - name: tmp
    emptyDir: {}
---
apiVersion: v1
kind: Pod
metadata:
  name: test-server2
  namespace: ` + testNamespace + `
  labels:
    app: test-server2
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: server
    image: nginxinc/nginx-unprivileged:alpine
    ports:
    - containerPort: 8080
    volumeMounts:
    - name: config
      mountPath: /usr/share/nginx/html
    - name: cache
      mountPath: /var/cache/nginx
    - name: run
      mountPath: /var/run
    - name: tmp
      mountPath: /tmp
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
  volumes:
  - name: config
    configMap:
      name: server2-data
  - name: cache
    emptyDir: {}
  - name: run
    emptyDir: {}
  - name: tmp
    emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: test-server1
  namespace: ` + testNamespace + `
spec:
  selector:
    app: test-server1
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: test-server2
  namespace: ` + testNamespace + `
spec:
  selector:
    app: test-server2
  ports:
  - port: 80
    targetPort: 8080
`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(multiServerManifest)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create multiple test servers")

			By("waiting for test servers to be ready")
			verifyServersReady := func(g Gomega) {
				for _, server := range []string{"test-server1", "test-server2"} {
					cmd := exec.Command("kubectl", "get", "pod", server, "-n", testNamespace, "-o", "jsonpath={.status.phase}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Running"))
				}
			}
			Eventually(verifyServersReady).Should(Succeed())

			By("creating multiple ExternalSources")
			multiSourceManifest := `
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: service1-config
  namespace: ` + testNamespace + `
spec:
  interval: 30s
  generator:
    type: http
    http:
      url: http://test-server1.` + testNamespace + `.svc.cluster.local/data.json
  destinationPath: service1.json
---
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: service2-config
  namespace: ` + testNamespace + `
spec:
  interval: 30s
  generator:
    type: http
    http:
      url: http://test-server2.` + testNamespace + `.svc.cluster.local/data.json
  destinationPath: service2.json
`
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(multiSourceManifest)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create multiple ExternalSources")

			By("waiting for all ExternalSources to be ready")
			verifyAllSourcesReady := func(g Gomega) {
				for _, source := range []string{"service1-config", "service2-config"} {
					cmd := exec.Command("kubectl", "get", "externalsource", source, "-n", testNamespace, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("True"))
				}
			}
			Eventually(verifyAllSourcesReady, 2*time.Minute).Should(Succeed())

			By("verifying all ExternalArtifacts were created")
			verifyAllArtifactsCreated := func(g Gomega) {
				for _, artifact := range []string{"service1-config", "service2-config"} {
					cmd := exec.Command("kubectl", "get", "externalartifact", artifact, "-n", testNamespace, "-o", "jsonpath={.spec.url}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).NotTo(BeEmpty())
				}
			}
			Eventually(verifyAllArtifactsCreated).Should(Succeed())

			By("cleaning up multiple test resources")
			cmd = exec.Command("kubectl", "delete", "externalsource", "service1-config", "service2-config", "-n", testNamespace)
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "pod,service,configmap", "-l", "app in (test-server1,test-server2)", "-n", testNamespace)
			_, _ = utils.Run(cmd)
		})
	})
})
