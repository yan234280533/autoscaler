/*
Copyright 2016 The Kubernetes Authors.

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

package volume

import (
	"cloud.tencent.com/tencent-cloudprovider/cbs-volume/lib/controller"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"cloud.tencent.com/tencent-cloudprovider/provider"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"

	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	"strings"
	"k8s.io/client-go/tools/record"
)

const (
	// are we allowed to set this? else make up our own
	annCreatedBy = "kubernetes.io/createdby"
	createdByCbs = "qcloud-cbs-dynamic-provisioner"

	// A PV annotation for the identity of the s3fsProvisioner that provisioned it
	annProvisionerID = "Provisioner_Id"

	QcloudCbsPluginName = "cloud.tencent.com/qcloud-cbs"
)

// NewFlexProvisioner creates a new flex provisioner
func NewFlexProvisioner(client kubernetes.Interface, eventRecorder  record.EventRecorder,
provider *qcloud.QCloud, clusterId string) controller.Provisioner {
	var identity types.UID

	provisioner := &cbsProvisioner{
		client:      client,
		eventRecorder:eventRecorder,
		provider: provider,
		identity:    identity,
		clusterId:clusterId,
	}

	return provisioner
}

func newFlexProvisionerInternal(client kubernetes.Interface, provider *qcloud.QCloud) *cbsProvisioner {
	var identity types.UID

	provisioner := &cbsProvisioner{
		client:      client,
		provider: provider,
		identity:    identity,
	}

	return provisioner
}

type cbsProvisioner struct {
	client        kubernetes.Interface
	eventRecorder record.EventRecorder

	provider      *qcloud.QCloud
	identity      types.UID
	clusterId     string
}

var _ controller.Provisioner = &cbsProvisioner{}




// Provision creates a volume i.e. the storage asset and returns a PV object for
// the volume.
func (p *cbsProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	diskId, volSize, labels, err := p.createVolume(options)
	if err != nil {
		return nil, err
	}

	annotations := make(map[string]string)
	annotations[annCreatedBy] = createdByCbs
	annotations[annProvisionerID] = string(p.identity)

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   options.PVName,
			Labels: labels,
			Annotations: annotations,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse(fmt.Sprintf("%dGi", volSize)),
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				QcloudCbs: &v1.QcloudCbsVolumeSource{
					CbsDiskId: diskId,
				},
			},
		},
	}

	return pv, nil
}

func (p *cbsProvisioner) GetAllZones() (sets.String, error) {
	zones := make(sets.String)
	nodes, err := p.client.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return zones, err
	}

	for _, node := range nodes.Items {
		if node.Labels != nil {
			zone, ok := node.Labels[kubeletapis.LabelZoneFailureDomain]
			if ok {
				zones.Insert(zone)
			}
		}
	}
	return zones, nil
}

func (p *cbsProvisioner) createVolume(options controller.VolumeOptions) (string, int, map[string]string, error) {

	var err error
	labels := make(map[string]string, 0)

	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestBytes := capacity.Value()

	volSizeGB := int(RoundUpSize(requestBytes, 1024 * 1024 * 1024))

	aspId := ""
	diskType := ""
	configuredZone := ""
	configuredZones := ""
	zonePresent := false
	zonesPresent := false
	for k, v := range options.Parameters {
		switch strings.ToLower(k) {
		case "aspid":
			aspId = v
		case "type":
			diskType = v
		case "zone":
			zonePresent = true
			configuredZone = v
		case "zones":
			zonesPresent = true
			configuredZones = v
		default:
			return "", 0, labels, fmt.Errorf("invalid option %q for volume plugin %s", k, p.provider.ProviderName())
		}
	}

	if zonePresent && zonesPresent {
		return "", 0, labels, fmt.Errorf("both zone and zones StorageClass parameters must not be used at the same time")
	}

	// TODO: implement PVC.Selector parsing
	if options.PVC.Spec.Selector != nil {
		return "", 0, labels, fmt.Errorf("claim.Spec.Selector is not supported for dynamic provisioning on GCE")
	}

	var zones sets.String
	if !zonePresent && !zonesPresent {
		zones, err = p.GetAllZones()
		if err != nil {
			glog.Errorf("error getting zone information from GCE: %v", err)
			return "", 0, labels, err
		}
	}
	if !zonePresent && zonesPresent {
		if zones, err = ZonesToSet(configuredZones); err != nil {
			return "", 0, labels, err
		}
	}
	if zonePresent && !zonesPresent {
		zones = make(sets.String)
		zones.Insert(configuredZone)
	}

	if zones.Len() == 0 {
		glog.Error("len(zones) == 0,may be no nodes")
		return "", 0, labels, fmt.Errorf("len(zones) == 0,may be no nodes")
	}

	zone := ChooseZoneForVolume(zones, options.PVC.Name)
	labels[kubeletapis.LabelZoneRegion] = p.provider.Config.Region
	labels[kubeletapis.LabelZoneFailureDomain] = zone

	diskId, size, err := p.provider.CreateDisk(diskType, volSizeGB, zone)
	if err != nil {
		glog.Errorf("Error creating qcloud cbs, size:%d, error: %v", volSizeGB, err)
		return "", 0, labels, err
	}
	p.provider.ModifyDiskName(diskId, fmt.Sprintf("%s/%s/%s", p.clusterId, options.PVC.Name, options.PVC.Namespace))

	if aspId != "" {
		err = p.provider.BindDiskToAsp(diskId, aspId)
		if err != nil {
			p.eventRecorder.Event(options.PVC, v1.EventTypeWarning, "BindAutoSnapshotPolicyFailed",
				fmt.Sprintf("Bind Disk(%s) to BindAutoSnapshotPolicy(%s) Failed,err:%s",
					diskId, aspId, err))
		}

	}

	return diskId, size, labels, nil
}

