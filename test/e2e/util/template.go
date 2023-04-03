// Copyright 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package util

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"time"
)

// NewLogSetTpl return a logSet template, name is random generated
func NewLogSetTpl(ns, image string) *v1alpha1.LogSet {
	l := &v1alpha1.LogSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "log-" + rand.String(6),
		},
		Spec: v1alpha1.LogSetSpec{
			LogSetBasic: v1alpha1.LogSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: image,
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("100Mi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					FileSystem: &v1alpha1.FileSystemProvider{
						Path: "/test",
					},
				},
				StoreFailureTimeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
	}
	return l
}

// NewSecretTpl return a secret template, which namespace is ns, and name is random generated
func NewSecretTpl(ns string) *corev1.Secret {
	sc := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "secret-" + rand.String(6),
		},
		Data: map[string][]byte{},
	}
	return sc
}

// SecretVolume return a volume has a secret as volumeSource, volume name is same to secret volume
func SecretVolume(name string) corev1.Volume {
	v := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
	return v
}
