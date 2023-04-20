// Copyright 2019-2023 The Liqo Authors
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

package statuslocal

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/strings/slices"

	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

const (
	ctrlPlaneCheckerName = "Control plane check"
)

var (
	liqoDeployments = []string{
		"liqo-controller-manager",
		"liqo-crd-replicator",
		"liqo-metric-agent",
		"liqo-auth",
		"liqo-proxy",
	}
	liqoDeploymentsNetwork = []string{
		"liqo-network-manager",
		"liqo-gateway",
	}
	liqoDaemonSetsNetwork = []string{
		"liqo-route",
	}
)

// errorCount is a struct holding a list of errors.
type errorCount struct {
	errors []error
}

// errorCountMap is a map with:
// key -> name of the pod;
// value -> errorCount for the given pod.
type errorCountMap map[string]*errorCount

type controllerType string

const (
	deployment controllerType = "Deployment"
	daemonSet  controllerType = "DaemonSet"
)

// componentState counts the number of pods in a particular state for a given deployment.
type componentState struct {
	// errorFlag set to true if errors are present.
	errorFlag bool

	// controllerType is the type of deployment.
	controllerType controllerType

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

func newComponentState(dType controllerType) componentState {
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

	outputTokens = append(outputTokens, pterm.Sprintf("Desired: %s", pterm.Bold.Sprint(ps.desired)))
	outputColor := pterm.FgGreen
	if ps.ready < ps.desired {
		outputColor = pterm.FgRed
	}
	outputTokens = append(outputTokens, pterm.Sprintf("Ready: %s",
		pterm.NewStyle(outputColor, pterm.Bold).Sprintf("%d/%d", ps.ready, ps.desired)))
	outputColor = pterm.FgGreen
	if ps.available < ps.desired {
		outputColor = pterm.FgRed
	}
	outputTokens = append(outputTokens, pterm.Sprintf("Available: %s",
		pterm.NewStyle(outputColor, pterm.Bold).Sprintf("%d/%d", ps.available, ps.desired)))
	return strings.Join(outputTokens, ", ")
}

// PodChecker implements the Check interface.
// holds the information about the control plane pods of Liqo.
type PodChecker struct {
	options           *status.Options
	deployments       []string
	daemonSets        []string
	podsState         podStateMap
	errors            bool
	collectionErrors  []error
	podCheckerSection output.Section
}

// NewPodChecker return a new pod checker.
func NewPodChecker(options *status.Options) *PodChecker {
	return &PodChecker{
		options:           options,
		podsState:         make(podStateMap, 6),
		errors:            false,
		podCheckerSection: output.NewRootSection(),
	}
}

// Silent implements the Checker interface.
func (pc *PodChecker) Silent() bool {
	return false
}

// Collect implements the collect method of the Checker interface.
// it collects the status of the components of Liqo. The status is
// collected at the pod level.
func (pc *PodChecker) Collect(ctx context.Context) {
	pc.deployments = slices.Clone(liqoDeployments)
	if pc.options.InternalNetworkEnabled {
		pc.deployments = append(pc.deployments, liqoDeploymentsNetwork...)
		pc.daemonSets = append(pc.daemonSets, liqoDaemonSetsNetwork...)
	}

	for _, dName := range pc.deployments {
		err := pc.deploymentStatus(ctx, dName)
		if err != nil {
			pc.addCollectionError(fmt.Errorf("unable to collect status for deployment %s in namespace %s: %w",
				dName, pc.options.LiqoNamespace, err,
			))
			pc.errors = true
		}
	}

	for _, dName := range pc.daemonSets {
		err := pc.daemonSetStatus(ctx, dName)
		if err != nil {
			pc.addCollectionError(fmt.Errorf("unable to collect status for daemonSet %s in namespace %s: %w",
				dName, pc.options.LiqoNamespace, err,
			))
			pc.errors = true
		}
	}
}

// HasSucceeded implements the HasSucceeded method of the Checker interface.
func (pc *PodChecker) HasSucceeded() bool {
	return !pc.errors
}

// GetTitle implements the GetTitle method of the Checker interface.
func (pc *PodChecker) GetTitle() string {
	return ctrlPlaneCheckerName
}

// Format implements the Format method of the Checker interface.
// it outputs the status of the Liqo components in a string ready to be
// printed out.
func (pc *PodChecker) Format() string {
	var text string

	dpmaxl := len(slice.LongestString(pc.deployments))
	dsmaxl := len(slice.LongestString(pc.daemonSets))
	var indent string

	dpAddSection := pc.podCheckerSection.AddSectionSuccess
	dsAddSection := pc.podCheckerSection.AddSectionSuccess
	var imgssec output.Section
	if pc.errors {
		for _, ps := range pc.podsState {
			if ps.errorFlag {
				switch ps.controllerType {
				case deployment:
					dpAddSection = pc.podCheckerSection.AddSectionFailure
				case daemonSet:
					dsAddSection = pc.podCheckerSection.AddSectionFailure
				}
			}
		}
	}
	dpsec := dpAddSection(string(deployment))
	var dssec output.Section
	if len(pc.daemonSets) != 0 {
		dssec = dsAddSection(string(daemonSet))
	}
	if pc.options.Verbose {
		imgssec = pc.podCheckerSection.AddSection("Images")
	}

	for name, dp := range pc.podsState {
		indent = ""
		switch dp.controllerType {
		case deployment:
			if dpmaxl < dsmaxl {
				indent = strings.Repeat(" ", dsmaxl-len(name))
			}
			dpsec.AddEntryWithoutStyle(name, indent+dp.format())
		case daemonSet:
			if dsmaxl < dpmaxl {
				indent = strings.Repeat(" ", dpmaxl-len(name))
			}
			dssec.AddEntryWithoutStyle(name, indent+dp.format())
		}

		if pc.options.Verbose {
			imgsec := imgssec.AddSection(name)
			for _, img := range dp.getImages() {
				imgsec.AddSectionInfo(img)
			}
		}
	}
	text = pc.podCheckerSection.SprintForBox(pc.options.Printer)

	if pc.errors {
		text += "\n\n"
		for name, podState := range pc.podsState {
			if podState.errorFlag {
				for pod, errorCol := range podState.errors {
					for _, err := range errorCol.errors {
						text += pc.options.Printer.Error.Sprintln(
							pc.options.Printer.Paragraph.Sprintf(
								"%s: %s, Pod: %s, Msg: %s",
								podState.controllerType, name, pod, err,
							))
					}
				}
			}
		}
		for _, err := range pc.collectionErrors {
			text += pc.options.Printer.Error.Sprintln(
				pc.options.Printer.Paragraph.Sprint(err.Error()),
			)
		}
		text = strings.TrimRight(text, "\n")
	}
	return text
}

// deploymentStatus collects the status of a given kubernetes Deployment.
func (pc *PodChecker) deploymentStatus(ctx context.Context, deploymentName string) error {
	var errors bool
	d, err := pc.options.KubeClient.AppsV1().Deployments(pc.options.LiqoNamespace).Get(ctx, deploymentName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if d == nil {
		return fmt.Errorf("deployment %s seems to be unavailable", deploymentName)
	}

	dState := newComponentState(deployment)
	dState.desired = int(d.Status.Replicas)
	dState.ready = int(d.Status.ReadyReplicas)
	dState.unavailable = int(d.Status.UnavailableReplicas)
	dState.available = int(d.Status.AvailableReplicas)

	// Get all the pods related with the current deployment.
	pods, err := pc.options.KubeClient.CoreV1().Pods(pc.options.LiqoNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(d.Spec.Selector.MatchLabels),
	})
	if err != nil {
		return err
	}

	if errors = checkPodsStatus(pods.Items, &dState); errors {
		pc.errors = errors
	}
	pc.podsState[deploymentName] = dState

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	return nil
}

// daemontSetStatus collects the status of a given kubernetes DaemonSet.
func (pc *PodChecker) daemonSetStatus(ctx context.Context, daemonSetName string) error {
	d, err := pc.options.KubeClient.AppsV1().DaemonSets(pc.options.LiqoNamespace).Get(ctx, daemonSetName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if d == nil {
		return fmt.Errorf("daemonSet %s seems to be unavailable", daemonSetName)
	}

	dState := newComponentState(daemonSet)
	dState.desired = int(d.Status.DesiredNumberScheduled)
	dState.ready = int(d.Status.NumberReady)
	dState.unavailable = int(d.Status.NumberUnavailable)
	dState.available = int(d.Status.NumberAvailable)

	// Get all the pods related with the current daemonset.
	pods, err := pc.options.KubeClient.CoreV1().Pods(pc.options.LiqoNamespace).List(ctx, metav1.ListOptions{
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
func (pc *PodChecker) addCollectionError(err error) {
	pc.collectionErrors = append(pc.collectionErrors, err)
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
