package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	remotecommandclient "k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

// ExecCmd executes a command inside a pod.
func ExecCmd(config *rest.Config, client kubernetes.Interface, podName, namespace, command string) (stdOut, stdErr string, retErr error) {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
		Command: cmd,
	}, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer
	executor, err := remotecommandclient.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	err = executor.Stream(remotecommandclient.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return stdout.String(), stderr.String(), err
}

// TriggerCheckNodeConnectivity checks nodePort service connectivity, executing a command for every node in the target cluster.
func TriggerCheckNodeConnectivity(localNodes *v1.NodeList, command string, nodePortValue int) error {
	if nodePortValue <= 0 {
		return fmt.Errorf("nodePort Value invalid (Must be >= 0)")
	}
	for index := range localNodes.Items {
		cmd := command + localNodes.Items[index].Status.Addresses[0].Address + ":" + strconv.Itoa(nodePortValue)
		c := exec.Command("sh", "-c", cmd) //nolint:gosec // Just a test, no need for this check
		output := &bytes.Buffer{}
		errput := &bytes.Buffer{}
		c.Stdout = output
		c.Stderr = errput
		klog.Infof("running command: %s", cmd)
		err := c.Run()
		if err != nil {
			klog.Error(err)
			klog.Info(output.String())
			klog.Info(errput.String())
			return err
		}
	}
	return nil
}
