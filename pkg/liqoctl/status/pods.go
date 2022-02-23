// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package status

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/utils/slice"
)

const (
	checkerName = "liqo-control-plane"
)

var (
	liqoDeployments = []string{
		"liqo-controller-manager",
		"liqo-network-manager",
		"liqo-crd-replicator",
		"liqo-webhook",
		"liqo-gateway",
		"liqo-auth",
	}
	liqoDaemonSets = []string{
		"liqo-route",
	}
)

// errorCount is a struct holding a list of errors.
type errorCount struct {
	errors []error
}

// collectionError struct holding the error for a given deployment of Liqo while collecting its status.
type collectionError struct {
	appType string
	appName string
	err     error
}

// newCollectionError return a new collectionError with the given arguments set.
func newCollectionError(appType, appName string, err error) collectionError {
	return collectionError{
		appType: appType,
		appName: appName,
		err:     err,
	}
}

// errorCountMap is a map with:
// key -> name of the pod;
// value -> errorCount for the given pod.
type errorCountMap map[string]*errorCount

// componentState counts the number of pods in a particular state for a given deployment.
type componentState struct {
	// errorFlag set to true if errors are present.
	errorFlag bool

	// controllerType is the type of deployment ("Deployment", "DaemonSet", ...).
	controllerType string

	// desired is the number of desired pods to be scheduled.
	desired int

	// ready is the number of ready pods.
	ready int

	// available is the number of available pods.
	available int

	// unavailable is the number of unavailable pods.
	unavailable int

	// imageVersions is the name of the images used by the pods (containers/init-containers), denoting the version of the images.
	imageVersions []string

	// errors is the aggregated errors of all pods.
	errors errorCountMap
}

// podStateMap is a map with:
// key -> name of the Liqo component;
// value -> componentState.
type podStateMap map[string]componentState

func newComponentState(dType string) componentState {
	return componentState{
		controllerType: dType,
		errors:         errorCountMap{},
	}
}

// getImages returns the images used by the current deployment/application.
func (ps *componentState) getImages() []string {
	return ps.imageVersions
}

// setImages sets the images passed as argument.
func (ps *componentState) setImages(images []string) {
	ps.imageVersions = images
}

// addImageVersion adds an image version to the existing ones.
func (ps *componentState) addImageVersion(imageVersion string) {
	iv := ps.getImages()
	if !(slice.ContainsString(iv, imageVersion)) {
		iv = append(iv, imageVersion)
	}
	ps.setImages(iv)
}

// getErrorCount returns the errorCount for given pod belonging to the current deployment/application.
func (ps *componentState) getErrorCount(pod string) *errorCount {
	errors := ps.errors
	// If the errorCount for the given pod does not exist then create and set it for the given pod.
	if errors[pod] == nil {
		errors[pod] = &errorCount{}
	}
	// Return the errorCount for the given pod managed by the given deployment.
	return errors[pod]
}

// addErrorForPod adds an error for a given pod belonging to che current deployment/application.
// At the same time the errorFlag is set to true. This way the components having an error could be
// discerned from the error free components.
func (ps *componentState) addErrorForPod(pod string, err error) {
	eCount := ps.getErrorCount(pod)
	eCount.errors = append(eCount.errors, err)
	// set error flag to true.
	ps.errorFlag = true
}

// format returns a string describing the status of the current deployment/application.
func (ps *componentState) format() string {
	var outputTokens []string

	if ps.desired > 0 {
		outputTokens = append(outputTokens, fmt.Sprintf("Desired: %d", ps.desired))
	}
	if ps.ready > 0 {
		outputColor := green
		if ps.ready < ps.desired {
			outputColor = yellow
		}
		outputTokens = append(outputTokens, fmt.Sprintf("Ready: "+outputColor+"%d/%d"+reset, ps.ready, ps.desired))
	}
	if ps.available > 0 {
		outputColor := green
		if ps.available < ps.desired {
			outputColor = yellow
		}
		outputTokens = append(outputTokens, fmt.Sprintf("available: "+outputColor+"%d/%d"+reset, ps.available, ps.desired))
	}
	if ps.unavailable > 0 {
		outputTokens = append(outputTokens, fmt.Sprintf(" Unavailable: "+red+"%d/%d"+reset, ps.unavailable, ps.desired))
	}
	outputTokens = append(outputTokens, fmt.Sprintf("Image version: %s", strings.Join(ps.getImages(), ", ")))
	return strings.Join(outputTokens, ", ")
}

// podChecker implements the Check interface.
// holds the information about the control plane pods of Liqo.
type podChecker struct {
	deployments      []string
	daemonSets       []string
	client           k8s.Interface
	namespace        string
	name             string
	podsState        podStateMap
	errors           bool
	collectionErrors []collectionError
}

// newPodChecker return a new pod checker.
func newPodChecker(namespace string, deployments, daemonSets []string, client k8s.Interface) *podChecker {
	return &podChecker{
		deployments: deployments,
		daemonSets:  daemonSets,
		client:      client,
		namespace:   namespace,
		name:        checkerName,
		podsState:   make(podStateMap, 6),
		errors:      false,
	}
}

// Collect implements the collect method of the Checker interface.
// it collects the status of the components of Liqo. The status is
// collected at the pod level.
func (pc *podChecker) Collect(ctx context.Context) error {
	for _, dName := range pc.deployments {
		err := pc.deploymentStatus(ctx, dName)
		if err != nil {
			pc.addCollectionError("Deployment", dName, fmt.Errorf("unable to collect status for deployment %s in namespace %s: %w", dName, pc.namespace, err))
			pc.errors = true
		}
	}

	for _, dName := range pc.daemonSets {
		err := pc.daemonSetStatus(ctx, dName)
		if err != nil {
			pc.addCollectionError("DaemonSet", dName, fmt.Errorf("unable to collect status for daemonSet %s in namespace %s: %w", dName, pc.namespace, err))
			pc.errors = true
		}
	}

	return nil
}

func (pc *podChecker) HasSucceeded() bool {
	return !pc.errors
}

// Format implements the format method of the Checker interface.
// it outputs the status of the Liqo components in a string ready to be
// printed out.
func (pc *podChecker) Format() (string, error) {
	w, buf := newTabWriter(pc.name)
	if pc.errors {
		// Add pod state to the buffer for each deployment type.
		if pc.errors {
			fmt.Fprintf(w, "%s liqo control plane is not OK\n", redCross)
			for deployment, podState := range pc.podsState {
				if podState.errorFlag {
					fmt.Fprintf(w, "%s\t%s\t%s\n", podState.controllerType, deployment, podState.format())
					for pod, errorCol := range podState.errors {
						for _, err := range errorCol.errors {
							fmt.Fprintf(w, "%s\t%s\tPod\t%s\t%s\n", podState.controllerType, deployment, pod, err)
						}
					}
				}
			}
			for _, err := range pc.collectionErrors {
				fmt.Fprintf(w, "%s\t%s\t%s\n", err.appType, err.appName, err.err)
			}
		}
	} else {
		fmt.Fprintf(w, "%s control plane pods are up and running\n", checkMark)
	}

	// Add a new line ad the end of the message.
	fmt.Fprintf(w, "\n")
	if err := w.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// deploymentStatus collects the status of a given kubernetes Deployment.
func (pc *podChecker) deploymentStatus(ctx context.Context, deploymentName string) error {
	var errors bool
	d, err := pc.client.AppsV1().Deployments(pc.namespace).Get(ctx, deploymentName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if d == nil {
		return fmt.Errorf("deployment %s seems to be unavailable", deploymentName)
	}

	dState := newComponentState("Deployment")
	dState.desired = int(d.Status.Replicas)
	dState.ready = int(d.Status.ReadyReplicas)
	dState.unavailable = int(d.Status.UnavailableReplicas)
	dState.available = int(d.Status.AvailableReplicas)

	// Get all the pods related with the current deployment.
	pods, err := pc.client.CoreV1().Pods(pc.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(d.Spec.Selector.MatchLabels),
	})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	if errors = checkPodsStatus(pods.Items, &dState); errors {
		pc.errors = errors
	}
	pc.podsState[deploymentName] = dState

	return nil
}

// daemontSetStatus collects the status of a given kubernetes DaemonSet.
func (pc *podChecker) daemonSetStatus(ctx context.Context, daemonSetName string) error {
	d, err := pc.client.AppsV1().DaemonSets(pc.namespace).Get(ctx, daemonSetName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if d == nil {
		return fmt.Errorf("daemonSet %s seems to be unavailable", daemonSetName)
	}

	dState := newComponentState("DaemonSet")
	dState.desired = int(d.Status.DesiredNumberScheduled)
	dState.ready = int(d.Status.NumberReady)
	dState.unavailable = int(d.Status.NumberUnavailable)
	dState.available = int(d.Status.NumberAvailable)

	// Get all the pods related with the current daemonset.
	pods, err := pc.client.CoreV1().Pods(pc.namespace).List(ctx, metav1.ListOptions{
		TypeMeta:      metav1.TypeMeta{},
		LabelSelector: labels.FormatLabels(d.Spec.Selector.MatchLabels),
	})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", daemonSetName)
	}

	if errors := checkPodsStatus(pods.Items, &dState); errors {
		pc.errors = errors
	}
	pc.podsState[daemonSetName] = dState
	return nil
}

// addCollectionError adds a collection error. A collection error is an error that happens while
// collecting the status of a Liqo component.
func (pc *podChecker) addCollectionError(deploymentType, deploymenName string, err error) {
	pc.collectionErrors = append(pc.collectionErrors, newCollectionError(deploymentType, deploymenName, err))
}

// checkPodsStatus fills the componentState data structure for a given pod.
// It returns a bool value which is set to true if the pod is not up and running, meaning there are errors.
func checkPodsStatus(podsList []corev1.Pod, dState *componentState) bool {
	var errorBool bool
	for i := range podsList {
		pod := podsList[i]
		podName := pod.Name
		// Collect images used by the containers of the current pod.
		for j := range pod.Spec.Containers {
			c := pod.Spec.Containers[j]
			dState.addImageVersion(c.Image)
		}
		// Collect images used by the init containers of the current pod.
		for j := range pod.Spec.InitContainers {
			c := pod.Spec.InitContainers[j]
			dState.addImageVersion(c.Image)
		}

		for l := range pod.Status.Conditions {
			cond := pod.Status.Conditions[l]
			switch cond.Type {
			case corev1.PodScheduled:
				if cond.Status != corev1.ConditionTrue {
					dState.addErrorForPod(podName, fmt.Errorf("not scheduled"))
					errorBool = true
				}
			case corev1.PodReady:
				if cond.Status != corev1.ConditionTrue {
					dState.addErrorForPod(podName, fmt.Errorf("not ready"))
					errorBool = true
				}
			case corev1.PodInitialized:
				if cond.Status != corev1.ConditionTrue {
					dState.addErrorForPod(podName, fmt.Errorf("not initialized"))
					errorBool = true
				}
			default:
			}
		}
	}
	return errorBool
}
