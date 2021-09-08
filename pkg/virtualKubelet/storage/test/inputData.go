// Copyright 2019-2021 The Liqo Authors
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

package test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
)

const (
	HomeNamespace    = "homeNamespace"
	ForeignNamespace = "foreignNamespace"

	Pod1 = "homePod1"
	Pod2 = "homePod2"
)

var (
	Pods = map[string]*corev1.Pod{
		utils.Keyer(HomeNamespace, Pod1): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod1,
				Namespace: HomeNamespace,
			},
		},
		utils.Keyer(HomeNamespace, Pod2): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod2,
				Namespace: HomeNamespace,
			},
		},

		utils.Keyer(ForeignNamespace, Pod1): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod1,
				Namespace: ForeignNamespace,
			},
		},
		utils.Keyer(ForeignNamespace, Pod2): {
			ObjectMeta: metav1.ObjectMeta{
				Name:      Pod2,
				Namespace: ForeignNamespace,
			},
		},
	}
)
