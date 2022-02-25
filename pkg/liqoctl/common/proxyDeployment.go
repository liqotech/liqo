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

package common

import (
	"context"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8s "k8s.io/client-go/kubernetes"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	proxyImage = "envoyproxy/envoy:v1.21.0"
)

var (
	proxyConfig = `
admin:
  address:
    socket_address:
      protocol: TCP
      address: 0.0.0.0
      port_value: 9901

static_resources:
  listeners:
  - name: listener_http
    address:
      socket_address:
        protocol: TCP
        address: 0.0.0.0
        port_value: 8118
    access_log:
      name: envoy.access_loggers.file
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.access_loggers.file.v3.FileAccessLog
        path: /dev/stdout
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains:
              - "*"
              routes:
              - match:
                  connect_matcher:
                    {}
                route:
                  cluster: api_server
                  upgrade_configs:
                  - upgrade_type: CONNECT
                    connect_config:
                      {}
          http_filters:
          - name: envoy.filters.http.router
  clusters:
  - name: api_server
    connect_timeout: 0.25s
    type: STRICT_DNS
    respect_dns_ttl: true
    dns_lookup_family: V4_ONLY
    dns_refresh_rate: 300s
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: api_server
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
               address: kubernetes.default
               port_value: 443`
)

func createProxyDeployment(ctx context.Context, cl k8s.Interface, name, ns string) (*Endpoint, error) {
	commonObjMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: ns,
		Labels: map[string]string{
			liqoconst.K8sAppNameKey: liqoconst.APIServerProxyAppName,
		},
	}

	proxyConfigMap := &corev1.ConfigMap{
		ObjectMeta: commonObjMeta,
		Data: map[string]string{
			"config": proxyConfig,
		},
	}

	proxyService := &corev1.Service{
		ObjectMeta: commonObjMeta,
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:        "http",
				Protocol:    "TCP",
				AppProtocol: nil,
				Port:        8118,
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 8118,
					StrVal: "8118",
				},
			}},
			Selector: map[string]string{
				liqoconst.K8sAppNameKey: liqoconst.APIServerProxyAppName,
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: commonObjMeta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					liqoconst.K8sAppNameKey: liqoconst.APIServerProxyAppName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: commonObjMeta.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "envoy",
							Image: proxyImage,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/envoy/envoy.yaml",
									SubPath:   "config",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: proxyName,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := cl.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	_, err = cl.CoreV1().ConfigMaps(ns).Create(ctx, proxyConfigMap, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	svc, err := cl.CoreV1().Services(ns).Create(ctx, proxyService, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}
	if k8serrors.IsAlreadyExists(err) {
		svc, err = cl.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return &Endpoint{
		ip:   svc.Spec.ClusterIP,
		port: strconv.FormatInt(int64(svc.Spec.Ports[0].Port), 10),
	}, err
}
