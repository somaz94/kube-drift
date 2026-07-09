/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/somaz94/kube-drift/test/utils"
)

const (
	// namespace where the operator is deployed.
	namespace = "kube-drift-system"
	// serviceAccountName created for the operator (namePrefix applied).
	serviceAccountName = "kube-drift-controller-manager"
	// metricsServiceName is the operator's metrics service (namePrefix applied).
	metricsServiceName = "kube-drift-controller-manager-metrics-service"
	// metricsReaderClusterRole allows GET on the /metrics endpoint (namePrefix applied).
	metricsReaderClusterRole = "kube-drift-metrics-reader"
	// metricsRoleBindingName is the ClusterRoleBinding created to read metrics.
	metricsRoleBindingName = "kube-drift-metrics-binding"
)

// driftManifests is applied to the workload cluster: a ConfigMap holding the
// desired manifests plus a DriftCheck pointing at it. The desired manifest
// describes a ConfigMap ("drift-target") that is deliberately never created in
// the cluster, so the controller must report exactly one "new" drift.
const driftManifests = `apiVersion: v1
kind: ConfigMap
metadata:
  name: desired-manifests
  namespace: default
data:
  manifests.yaml: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: drift-target
      namespace: default
    data:
      key: desired-value
---
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: drift-e2e
  namespace: default
spec:
  source:
    type: ConfigMap
    configMap:
      name: desired-manifests
  target:
    namespaces:
      - default
  interval: 15s
`

// kustomizeDriftCheck points a Kustomize source at the in-repo fixture overlay,
// cloned anonymously over Git. The overlay renders "e2e-kustomize-target"
// (namePrefix applied), which is never created, so exactly one "new" drift is
// reported — proving the in-cluster Git clone + in-process kustomize build path.
const kustomizeDriftCheck = `apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: drift-kustomize-e2e
  namespace: default
spec:
  source:
    type: Kustomize
    kustomize:
      git:
        url: https://github.com/somaz94/kube-drift.git
        ref: main
        path: test/e2e/fixtures/kustomize
  interval: 15s
`

// helmDriftCheck points a Helm source at the in-repo fixture chart, cloned over
// Git. The chart renders "hc-target" (from .Release.Name), never created, so one
// "new" drift is reported — proving the in-cluster Git clone + in-process Helm
// render path.
const helmDriftCheck = `apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: drift-helm-e2e
  namespace: default
spec:
  source:
    type: Helm
    helm:
      git:
        url: https://github.com/somaz94/kube-drift.git
        ref: main
        path: test/e2e/fixtures/helm
      releaseName: hc
  interval: 15s
`

// notifyFixture deploys an HTTP echo receiver plus a drifting DriftCheck whose
// notify webhook targets it. The echo pod logs each request body, so the test
// can assert the Generic notification payload (carrying the DriftCheck name)
// was delivered — proving the end-to-end webhook-notification path.
const notifyFixture = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: drift-echo
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: drift-echo
  template:
    metadata:
      labels:
        app: drift-echo
    spec:
      containers:
        - name: echo
          image: mendhak/http-https-echo:31
          env:
            - name: HTTP_PORT
              value: "8080"
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: drift-echo
  namespace: default
spec:
  selector:
    app: drift-echo
  ports:
    - port: 8080
      targetPort: 8080
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: notify-desired
  namespace: default
data:
  m.yaml: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: notify-drift-target
      namespace: default
    data:
      key: v
---
apiVersion: drift.somaz.io/v1alpha1
kind: DriftCheck
metadata:
  name: drift-notify-e2e
  namespace: default
spec:
  source:
    type: ConfigMap
    configMap:
      name: notify-desired
  notify:
    webhooks:
      - type: Generic
        url: http://drift-echo.default.svc.cluster.local:8080/
  interval: 15s
`

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	AfterAll(func() {
		By("deleting the DriftCheck and desired manifests")
		cmd := exec.Command("kubectl", "delete", "-f", driftManifestsPath(), "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("removing the metrics ClusterRoleBinding")
		cmd = exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("cleaning up the curl pod for metrics")
		cmd = exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should detect drift and serve the kube_drift_resources metric", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole="+metricsReaderClusterRole,
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("applying the desired manifests and a DriftCheck that must report drift")
			Expect(os.WriteFile(driftManifestsPath(), []byte(driftManifests), 0o644)).To(Succeed())
			DeferCleanup(func() { _ = os.Remove(driftManifestsPath()) })
			cmd = exec.Command("kubectl", "apply", "-f", driftManifestsPath())
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply DriftCheck")

			By("waiting for the DriftCheck to report exactly one new drift")
			verifyDrift := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "driftcheck", "drift-e2e", "-n", "default",
					"-o", "jsonpath={.status.summary.new}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "expected status.summary.new=1")
			}
			Eventually(verifyDrift).Should(Succeed())

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:7.78.0",
				"--", "/bin/sh", "-c", fmt.Sprintf(
					"curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics",
					token, metricsServiceName, namespace))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("verifying the metrics output exposes both runtime and kube-drift metrics")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring("controller_runtime_reconcile_total"))
			Expect(metricsOutput).To(ContainSubstring("kube_drift_resources"),
				"the custom kube_drift_resources gauge should be exposed after a drift evaluation")
		})

		It("should detect drift from a Kustomize source cloned over Git", func() {
			applyManifest("kube-drift-e2e-kustomize.yaml", kustomizeDriftCheck)

			By("waiting for the rendered kustomize overlay to report exactly one new drift")
			verifyKustomizeDrift := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "driftcheck", "drift-kustomize-e2e", "-n", "default",
					"-o", "jsonpath={.status.summary.new}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("1"), "expected the rendered e2e-kustomize-target to be reported new")
			}
			Eventually(verifyKustomizeDrift, 3*time.Minute).Should(Succeed())
		})

		It("should detect drift from a Helm source cloned over Git", func() {
			applyManifest("kube-drift-e2e-helm.yaml", helmDriftCheck)

			By("waiting for the rendered helm chart to report exactly one new drift")
			verifyHelmDrift := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "driftcheck", "drift-helm-e2e", "-n", "default",
					"-o", "jsonpath={.status.summary.new}")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("1"), "expected the rendered hc-target to be reported new")
			}
			Eventually(verifyHelmDrift, 3*time.Minute).Should(Succeed())
		})

		It("should deliver a webhook notification when drift is detected", func() {
			applyManifest("kube-drift-e2e-notify.yaml", notifyFixture)

			By("waiting for the echo receiver to become available")
			verifyEchoUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "rollout", "status", "deploy/drift-echo",
					"-n", "default", "--timeout=10s")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyEchoUp, 2*time.Minute).Should(Succeed())

			By("waiting for the notification payload to reach the echo receiver")
			verifyNotified := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", "deploy/drift-echo", "-n", "default")
				out, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(ContainSubstring("drift-notify-e2e"),
					"the Generic webhook payload naming the DriftCheck should have been POSTed to the receiver")
			}
			Eventually(verifyNotified, 3*time.Minute).Should(Succeed())
		})
	})
})

// applyManifest writes content to a temp file, applies it, and registers a
// deferred delete + cleanup so each scenario tears down its own fixture.
func applyManifest(name, content string) {
	path := filepath.Join(os.TempDir(), name)
	ExpectWithOffset(1, os.WriteFile(path, []byte(content), 0o644)).To(Succeed())
	DeferCleanup(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", path, "--ignore-not-found"))
		_ = os.Remove(path)
	})
	_, err := utils.Run(exec.Command("kubectl", "apply", "-f", path))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "failed to apply "+name)
}

// driftManifestsPath returns the temp-file path used to apply and delete the
// DriftCheck fixture.
func driftManifestsPath() string {
	return filepath.Join(os.TempDir(), "kube-drift-e2e-driftcheck.yaml")
}

// serviceAccountToken returns a token for the operator's service account using
// the Kubernetes TokenRequest API.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join(os.TempDir(), secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tokenRequestFile) }()

	var out string
	verifyTokenCreation := func(g Gomega) {
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		// Use Output (not CombinedOutput) so a kubectl warning on stderr cannot
		// prepend non-JSON text and break the Unmarshal below.
		output, err := cmd.Output()
		g.Expect(err).NotTo(HaveOccurred())

		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput returns the curl-metrics pod logs and asserts the request
// succeeded (HTTP 200).
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a minimal view of the Kubernetes TokenRequest API response.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
