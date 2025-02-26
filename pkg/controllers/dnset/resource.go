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

package dnset

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/pkg/errors"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

const (
	serviceType = "DN"
)

// dn service entrypoint script
var startScriptTpl = template.Must(template.New("dn-start-script").Parse(`
#!/bin/sh
set -eu
POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
ORDINAL=${POD_NAME##*-}
if [ -z "${HOSTNAME_UUID+guard}" ]; then
  UUID=$(printf '00000000-0000-0000-0000-1%011x' ${ORDINAL})
else
  UUID=$(echo ${ADDR} | sha256sum | od -x | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
fi
conf=$(mktemp)
bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
service-address = "${ADDR}:{{ .DNServicePort }}"
EOF
# build instance config
sed "/\[dn\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

# append lock-service configs
lsc=$(mktemp)
cat <<EOF > ${lsc}
service-address = "${ADDR}:{{ .LockServicePort }}"
EOF
sed -i "/\[dn.lockservice\]/r ${lsc}" ${conf}

# there is a chance that the dns is not yet added to kubedns and the
# server will crash, wait before myself to be resolvable
elapseTime=0
period=1
threshold=30
while true; do
    sleep ${period}
    elapseTime=$(( elapseTime+period ))
    if [ ${elapseTime} -ge ${threshold} ]; then
        echo "waiting for dns resolvable timeout" >&2 && exit 1
    fi
    if nslookup ${ADDR} >/dev/null; then
        break
    else
        echo "waiting pod dns name ${ADDR} resolvable" >&2
    fi
done

echo "/mo-service -cfg ${conf}"
exec /mo-service -cfg ${conf}
`))

type model struct {
	DNServicePort  int
	ConfigFilePath string

	LockServicePort int
}

func syncReplicas(dn *v1alpha1.DNSet, cs *kruise.StatefulSet) {
	cs.Spec.Replicas = &dn.Spec.Replicas
}

func syncPodMeta(dn *v1alpha1.DNSet, cs *kruise.StatefulSet) {
	dn.Spec.Overlay.OverlayPodMeta(&cs.Spec.Template.ObjectMeta)
}

func syncPodSpec(dn *v1alpha1.DNSet, sts *kruise.StatefulSet, sp v1alpha1.SharedStorageProvider) {
	volumeMountsList := []corev1.VolumeMount{
		{
			Name:      common.ConfigVolume,
			ReadOnly:  true,
			MountPath: common.ConfigPath,
		},
	}

	dataVolume := corev1.VolumeMount{
		Name:      common.DataVolume,
		MountPath: common.DataPath,
	}

	if dn.Spec.CacheVolume != nil {
		volumeMountsList = append(volumeMountsList, dataVolume)
	}
	mainRef := util.FindFirst(sts.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}

	mainRef.Image = dn.Spec.Image
	mainRef.Resources = dn.Spec.Resources
	mainRef.Command = []string{
		"/bin/sh", fmt.Sprintf("%s/%s", common.ConfigPath, common.Entrypoint),
	}
	mainRef.VolumeMounts = volumeMountsList
	mainRef.Env = []corev1.EnvVar{
		util.FieldRefEnv(common.PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(common.NamespaceEnvKey, "metadata.namespace"),
		{Name: common.HeadlessSvcEnvKey, Value: headlessSvcName(dn)},
	}

	if dn.Spec.DNSBasedIdentity {
		mainRef.Env = append(mainRef.Env, corev1.EnvVar{Name: "HOSTNAME_UUID", Value: "y"})
	}
	dn.Spec.Overlay.OverlayMainContainer(mainRef)
	specRef := &sts.Spec.Template.Spec
	specRef.Containers = []corev1.Container{*mainRef}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = dn.Spec.NodeSelector

	common.SetStorageProviderConfig(sp, specRef)
	common.SyncTopology(dn.Spec.TopologyEvenSpread, specRef)

	dn.Spec.Overlay.OverlayPodSpec(specRef)
}

// buildDNSetConfigMap return dn set configmap
func buildDNSetConfigMap(dn *v1alpha1.DNSet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	if ls.Status.Discovery == nil {
		return nil, errors.New("HAKeeper discovery address not ready")
	}
	conf := dn.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	conf.Set([]string{"hakeeper-client", "service-addresses"}, logset.HaKeeperAdds(ls))
	// conf.Set([]string{"hakeeper-client", "discovery-address"}, ls.Status.Discovery.String())
	conf.Merge(common.FileServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage, dn.Spec.CacheVolume, &dn.Spec.SharedStorageCache))
	conf.Set([]string{"service-type"}, serviceType)
	conf.Set([]string{"dn", "listen-address"}, getListenAddress())
	conf.Set([]string{"dn", "lockservice", "listen-address"}, fmt.Sprintf("0.0.0.0:%d", common.LockServicePort))
	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}

	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		DNServicePort:   dnServicePort,
		LockServicePort: common.LockServicePort,
		ConfigFilePath:  fmt.Sprintf("%s/%s", common.ConfigPath, common.ConfigFile),
	})
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: common.ObjMetaTemplate(dn, configMapName(dn)),
		Data: map[string]string{
			common.ConfigFile: s,
			common.Entrypoint: buff.String(),
		},
	}, nil
}

func buildHeadlessSvc(dn *v1alpha1.DNSet) *corev1.Service {
	return common.HeadlessServiceTemplate(dn, headlessSvcName(dn))
}

func buildDNSet(dn *v1alpha1.DNSet) *kruise.StatefulSet {
	return common.StatefulSetTemplate(dn, stsName(dn), headlessSvcName(dn))
}

func syncPersistentVolumeClaim(dn *v1alpha1.DNSet, sts *kruise.StatefulSet) {
	if dn.Spec.CacheVolume != nil {
		dataPVC := common.PersistentVolumeClaimTemplate(dn.Spec.CacheVolume.Size, dn.Spec.CacheVolume.StorageClassName, common.DataVolume)
		tpls := []corev1.PersistentVolumeClaim{dataPVC}
		dn.Spec.Overlay.AppendVolumeClaims(&tpls)
		sts.Spec.VolumeClaimTemplates = tpls
	}
}

func syncPods(ctx *recon.Context[*v1alpha1.DNSet], sts *kruise.StatefulSet) error {
	cm, err := buildDNSetConfigMap(ctx.Obj, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}

	syncPodMeta(ctx.Obj, sts)
	if ctx.Dep != nil {
		syncPodSpec(ctx.Obj, sts, ctx.Dep.Deps.LogSet.Spec.SharedStorage)

	}

	return common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm)
}
