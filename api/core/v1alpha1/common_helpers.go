// Copyright 2023 Matrix Origin
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

package v1alpha1

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	reasonEmpty = "empty"

	// defaultArgsFile is the field name in matrixone-operator-cm configmap, contains information of default service args
	defaultArgsFile = "defaultArgs"

	BucketUniqLabel = "matrixorigin.io/bucket-unique-id"
)

var (
	// ServiceDefaultArgs is a cache variable for default args, should be read only in this package
	ServiceDefaultArgs *DefaultArgs
)

func (c *ConditionalStatus) SetCondition(condition metav1.Condition) {
	if c.Conditions == nil {
		c.Conditions = []metav1.Condition{}
	}
	if condition.Reason == "" {
		condition.Reason = reasonEmpty
	}
	meta.SetStatusCondition(&c.Conditions, condition)
}

func (c *ConditionalStatus) GetConditions() []metav1.Condition {
	if c == nil {
		return nil
	}
	return c.Conditions
}

func (o *Overlay) OverlayPodMeta(meta *metav1.ObjectMeta) {
	if o == nil {
		return
	}
	if o.PodLabels != nil {
		// we are risking overwrite original labels here, this is desirable since overlay is
		// for advanced use-case and we should allow fine-grained (through risky) control
		for k, v := range o.PodLabels {
			meta.Labels[k] = v
		}
	}
	if o.PodAnnotations != nil {
		for k, v := range o.PodAnnotations {
			meta.Annotations[k] = v
		}
	}
}

// AppendVolumeClaims append the volume claims to the given claims
func (o *Overlay) AppendVolumeClaims(claims *[]corev1.PersistentVolumeClaim) {
	if o == nil {
		return
	}
	// TODO(aylei): maybe we need to append the overlay volume claims to the volume claim template
	*claims = append(*claims, o.VolumeClaims...)
}

func (o *Overlay) OverlayPodSpec(pod *corev1.PodSpec) {
	if o == nil {
		return
	}
	if o.Volumes != nil {
		pod.Volumes = util.UpsertListByKey(pod.Volumes, o.Volumes, func(v corev1.Volume) string {
			return v.Name
		})
	}
	if o.Affinity != nil {
		pod.Affinity = o.Affinity
	}
	if o.ServiceAccountName != "" {
		pod.ServiceAccountName = o.ServiceAccountName
	}
	if o.SecurityContext != nil {
		pod.SecurityContext = o.SecurityContext
	}
	if o.ImagePullSecrets != nil {
		pod.ImagePullSecrets = o.ImagePullSecrets
	}
	if o.Tolerations != nil {
		pod.Tolerations = o.Tolerations
	}
	if o.PriorityClassName != "" {
		pod.PriorityClassName = o.PriorityClassName
	}
	if o.TerminationGracePeriodSeconds != nil {
		pod.TerminationGracePeriodSeconds = o.TerminationGracePeriodSeconds
	}
	if o.HostAliases != nil {
		pod.HostAliases = o.HostAliases
	}
	if o.TopologySpreadConstraints != nil {
		// overwrite any pre-generated topologySpreadConstraints if an overlay is set
		pod.TopologySpreadConstraints = o.TopologySpreadConstraints
	}
	if o.RuntimeClassName != nil {
		pod.RuntimeClassName = o.RuntimeClassName
	}
	if o.DNSConfig != nil {
		pod.DNSConfig = o.DNSConfig
	}
	if o.InitContainers != nil {
		// overwrite init containers if an overlay is set
		pod.InitContainers = o.InitContainers
	}
	if o.SidecarContainers != nil {
		// overwrite all containers except "main" if an overlay is set
		var containers []corev1.Container
		main := findMainContainer(pod.Containers)
		if main != nil {
			containers = append(containers, *main)
		}
		containers = append(containers, o.SidecarContainers...)
		pod.Containers = containers
	}
}

func (o *Overlay) OverlayMainContainer(c *corev1.Container) {
	if o == nil {
		return
	}
	mc := o.MainContainerOverlay
	if mc.ImagePullPolicy != nil {
		c.ImagePullPolicy = *o.ImagePullPolicy
	}
	if mc.Command != nil {
		c.Command = mc.Command
	}
	if mc.Args != nil {
		c.Args = mc.Args
	}
	if mc.EnvFrom != nil {
		c.EnvFrom = mc.EnvFrom
	}
	if mc.Env != nil {
		c.Env = util.UpsertListByKey(c.Env, mc.Env, func(v corev1.EnvVar) string {
			return v.Name
		})
	}
	if mc.ReadinessProbe != nil {
		c.ReadinessProbe = mc.ReadinessProbe
	}
	if mc.LivenessProbe != nil {
		c.LivenessProbe = mc.LivenessProbe
	}
	if mc.Lifecycle != nil {
		c.Lifecycle = mc.Lifecycle
	}
	if mc.VolumeMounts != nil {
		c.VolumeMounts = util.UpsertListByKey(c.VolumeMounts, o.VolumeMounts, func(v corev1.VolumeMount) string {
			return v.Name
		})
	}
}

func (s *FailoverStatus) StoresFailedFor(d time.Duration) []Store {
	var stores []Store

	for _, store := range s.FailedStores {
		if time.Now().Sub(store.LastTransitionTime.Time) >= d {
			stores = append(stores, store)
		}
	}

	return stores
}

func findMainContainer(containers []corev1.Container) *corev1.Container {
	for _, c := range containers {
		if c.Name == ContainerMain {
			return &c
		}
	}
	return nil
}

// DefaultArgs contain default service args for logservice/dn/tp, these default args set in matrixone-operator-cm configmap
type DefaultArgs struct {
	LogService []string `json:"logService,omitempty"`
	DN         []string `json:"dn,omitempty"`
	CN         []string `json:"cn,omitempty"`
}

// setDefaultServiceArgs set default args for service, we only set default args when there is service args config in service spec
func setDefaultServiceArgs(object interface{}) {
	if ServiceDefaultArgs == nil {
		return
	}
	switch obj := object.(type) {
	case *LogSetBasic:
		// set default arguments only when user does not set any arguments
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.LogService
		}
	case *DNSetBasic:
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.DN
		}
	case *CNSetBasic:
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.CN
		}
	default:
		moLog.Error(fmt.Errorf("unknown type:%T", object), "expected types: *LogSetBasic, *DNSetBasic, *CNSetBasic")
		return
	}
}

// ClaimedBucket return claimed bucket according to S3Provider configuration, caller must ensure that provider is not nil
// NOTE: ClaimedBucket search bucket in cluster scope
func ClaimedBucket(c client.Client, provider *S3Provider) (*BucketClaim, error) {
	uniqLabel := UniqueBucketLabel(provider)
	bcList := &BucketClaimList{}
	if err := c.List(context.TODO(), bcList, client.MatchingLabels{BucketUniqLabel: uniqLabel}); err != nil {
		return nil, err
	}

	switch len(bcList.Items) {
	case 0:
		return nil, nil
	case 1:
		return &bcList.Items[0], nil
	default:
		return nil, fmt.Errorf("list more than one buckets")
	}
}

// UniqueBucketLabel generate an unique id for S3 provider, this id becomes a label in bucketClaim
func UniqueBucketLabel(s3Provider *S3Provider) string {
	var providerType string
	if s3Provider.Type != nil {
		providerType = string(*s3Provider.Type)
	}
	uniqId := fmt.Sprintf("%s-%s-%s", providerType, s3Provider.Endpoint, s3Provider.Path)
	uniqId = fmt.Sprintf("%x", sha1.Sum([]byte(uniqId)))
	return uniqId
}

func BucketBindToMark(logsetMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%s/%s", logsetMeta.Namespace, logsetMeta.Name)
}

func LogSetKey(mo *MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name,
		Namespace: mo.Namespace,
	}
}

func DNSetKey(mo *MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name,
		Namespace: mo.Namespace,
	}
}

func TPSetKey(mo *MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-tp",
		Namespace: mo.Namespace,
	}
}

func APSetKey(mo *MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-ap",
		Namespace: mo.Namespace,
	}
}

func WebUIKey(mo *MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name,
		Namespace: mo.Namespace,
	}
}
