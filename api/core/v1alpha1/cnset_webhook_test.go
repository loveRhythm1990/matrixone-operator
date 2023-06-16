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

package v1alpha1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CNSet Webhook", func() {

	It("should accept CNSet of old versions", func() {
		// DO NOT mutate the following spec.
		// This spec is valid in mo-operator v0.6.0 and should always be accepted by
		// the webhook for backward compatibility.
		v06 := &CNSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cn-" + randomString(5),
				Namespace: "default",
			},
			Spec: CNSetSpec{
				PodSet: PodSet{
					Replicas: 2,
					MainContainer: MainContainer{
						Image: "test",
					},
				},
			},
			Deps: CNSetDeps{
				LogSetRef: LogSetRef{
					LogSet: &LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), v06)).To(Succeed())
	})

	It("should set default cache size", func() {
		cn := &CNSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cn-" + randomString(5),
				Namespace: "default",
			},
			Spec: CNSetSpec{
				PodSet: PodSet{
					Replicas: 2,
					MainContainer: MainContainer{
						Image: "test",
					},
				},
				CacheVolume: &Volume{
					Size: resource.MustParse("20Gi"),
				},
			},
			Deps: CNSetDeps{
				LogSetRef: LogSetRef{
					ExternalLogSet: &ExternalLogSet{},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), cn)).To(Succeed())
		expected := resource.MustParse("18Gi")
		Expect(cn.Spec.SharedStorageCache.DiskCacheSize.Value()).To(Equal(expected.Value()))
	})

	It("should reject empty CN label key or values", func() {
		cnTpl := &CNSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cn-" + randomString(5),
				Namespace: "default",
			},
			Spec: CNSetSpec{
				PodSet: PodSet{
					Replicas: 2,
					MainContainer: MainContainer{
						Image: "test",
					},
				},
			},
			Deps: CNSetDeps{
				LogSetRef: LogSetRef{
					ExternalLogSet: &ExternalLogSet{},
				},
			},
		}
		emptyLabelKey := cnTpl.DeepCopy()
		emptyLabelKey.Spec.Labels = []CNLabel{{
			Key:    "",
			Values: []string{"test"},
		}}
		Expect(k8sClient.Create(context.TODO(), emptyLabelKey)).NotTo(Succeed())
		emptyLabelValueList := cnTpl.DeepCopy()
		emptyLabelValueList.Spec.Labels = []CNLabel{{
			Key:    "test",
			Values: []string{},
		}}
		Expect(k8sClient.Create(context.TODO(), emptyLabelValueList)).NotTo(Succeed())
		emptyLabelValueItem := cnTpl.DeepCopy()
		emptyLabelValueItem.Spec.Labels = []CNLabel{{
			Key:    "test",
			Values: []string{""},
		}}
		Expect(k8sClient.Create(context.TODO(), emptyLabelValueItem)).NotTo(Succeed())
		validLabel := cnTpl.DeepCopy()
		emptyLabelValueItem.Spec.Labels = []CNLabel{{
			Key:    "test",
			Values: []string{"test"},
		}}
		Expect(k8sClient.Create(context.TODO(), validLabel)).To(Succeed())
	})
})
