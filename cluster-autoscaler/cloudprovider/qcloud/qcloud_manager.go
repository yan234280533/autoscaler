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


package qcloud

import (
	"fmt"
	"io"
	"time"

	autoscaling "github.com/dbdd4us/qcloudapi-sdk-go/scaling"
	"github.com/dbdd4us/qcloudapi-sdk-go/ccs"
	"github.com/dbdd4us/qcloudapi-sdk-go/common"
	"github.com/golang/glog"
	"cloud.tencent.com/tencent-cloudprovider/credential"
	"encoding/json"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
)

type asgInformation struct {
	config   *Asg
	basename string
}

// QcloudManager is handles qcloud communication and data caching.
type QcloudManager struct {
	service autoScalingWrapper
	asgs    *autoScalingGroups
}

type Config struct {
	Region string `json:"region"`
	Zone   string `json:"zone"`
}

const (
	LABEL_AUTO_SCALING_GROUP_ID = "cloud.tencent.com/auto-scaling-group-id"
)

var config Config

func readConfig(cfg io.Reader) error {
	if cfg == nil {
		err := fmt.Errorf("No cloud provider config given")
		return err
	}

	if err := json.NewDecoder(cfg).Decode(&config); err != nil {
		glog.Errorf("Couldn't parse config: %v", err)
		return err
	}

	return nil
}

// CreateQcloudManager constructs qcloudManager object.
func CreateQcloudManager(configReader io.Reader) (*QcloudManager, error) {
	if configReader == nil {
		glog.Errorf("qcloud need set config")
		return nil, fmt.Errorf("qcloud need set config")
	}

	err := readConfig(configReader)
	if err != nil {
		return nil, err
	}

	refresher, err := credential.NewNormRefresher(time.Second * 3600)
	if err != nil {
		return nil, err
	}

	normCredential, err := credential.NewNormCredential(time.Second * 3600, refresher)
	if err != nil {
		glog.Errorf("NewNormCredential error")
		return nil, err
	}

	cli, err := autoscaling.NewClient(&normCredential, common.Opts{Region: config.Region})
	if err != nil {
		glog.Errorf("qcloud api client error")
		return nil, err
	}

	ccsCli, err := ccs.NewClient(&normCredential, common.Opts{Region: config.Region})
	if err != nil {
		glog.Errorf("qcloud ccs api client error")
		return nil, err
	}

	service := autoScalingWrapper{
		Client:cli,
		CcsClient:ccsCli,
	}
	manager := &QcloudManager{
		asgs:    newAutoScalingGroups(service),
		service: service,
	}

	return manager, nil
}

// RegisterAsg registers asg in Qcloud Manager.
func (m *QcloudManager) RegisterAsg(asg *Asg) {
	m.asgs.Register(asg)
}

// GetAsgForInstance returns AsgConfig of the given Instance
func (m *QcloudManager) GetAsgForInstance(instance *QcloudRef) (*Asg, error) {
	return m.asgs.FindForInstance(instance)
}



// GetAsgSize gets ASG size.
func (m *QcloudManager) GetAsgSize(asgConfig *Asg) (int64, error) {
	params := &autoscaling.DescribeScalingGroupArgs{
		ScalingGroupIds: []string{asgConfig.Name},
	}
	groups, err := m.service.Client.DescribeScalingGroup(params)

	if err != nil {
		return -1, err
	}

	if len(groups.Data.ScalingGroupSet) < 1 {
		return -1, fmt.Errorf("Unable to get first autoscaling.Group for %s", asgConfig.Name)
	}

	return groups.Data.ScalingGroupSet[0].DesiredCapacity, nil
}

// SetAsgSize sets ASG size.
func (m *QcloudManager) SetAsgSize(asg *Asg, size int64) error {
	params := &autoscaling.ModifyScalingGroupArgs{
		ScalingGroupId: asg.Name,
		DesiredCapacity:size,
	}
	glog.V(0).Infof("Setting asg %s size to %d", asg.Id(), size)
	_, err := m.service.Client.ModifyScalingGroup(params)
	if err != nil {
		return err
	}
	return nil
}

// DeleteInstances deletes the given instances. All instances must be controlled by the same ASG.
func (m *QcloudManager) DeleteInstances(instances []*QcloudRef) error {
	if len(instances) == 0 {
		return nil
	}
	commonAsg, err := m.asgs.FindForInstance(instances[0])
	if err != nil {
		return err
	}
	for _, instance := range instances {
		asg, err := m.asgs.FindForInstance(instance)
		if err != nil {
			return err
		}
		if asg != commonAsg {
			return fmt.Errorf("Connot delete instances which don't belong to the same ASG.")
		}
	}

	var ins []string
	for _, instance := range instances {
		ins = append(ins, instance.Name)
	}

	params := &autoscaling.DetachInstanceArgs{
		ScalingGroupId:commonAsg.Name,
		InstanceIds:ins,
	}
	resp, err := m.service.Client.DetachInstance(params)
	if err != nil {
		return err
	}
	glog.V(4).Infof("res:%#v", resp.Response)

	return nil
}

// GetAsgNodes returns Asg nodes.
func (m *QcloudManager) GetAsgNodes(asg *Asg) ([]string, error) {
	result := make([]string, 0)
	group, err := m.service.getAutoscalingInstance(asg.Name)
	if err != nil {
		return []string{}, err
	}
	for _, instance := range group {
		if instance.LifeCycleState == "Removing" {
			continue
		}
		result = append(result, instance.InstanceId)
			//fmt.Sprintf("qcloud:///%s/%s", config.Zone, instance.InstanceId))
	}
	return result, nil
}

type asgTemplate struct {
	InstanceType string
	Region       string
	Zone         string
	Cpu          int
	Mem          int
	Label        map[string]string
}

func (m *QcloudManager) getAsgTemplate(name string) (*asgTemplate, error) {
	asg, err := m.service.getAutoscalingGroupByName(name)
	if err != nil {
		return nil, err
	}

	cpu, mem, err := m.service.getInstanceTypeByLCName(asg.ScalingConfigurationId)
	if err != nil {
		return nil, err
	}

	asgLabel, err := m.service.getAutoscalingGroupLabel(name)
	if err != nil {
		return nil, err
	}

	asgLabel.Label[LABEL_AUTO_SCALING_GROUP_ID] = name

	if len(asg.SubnetIdSet) < 1 {
		return nil, fmt.Errorf("Unable to get first AvailabilityZone for %s", name)
	}

	az := fmt.Sprintf("%d", asg.SubnetIdSet[0].ZoneId)

	if len(asg.SubnetIdSet) > 1 {
		glog.Warningf("Found multiple availability zones, using %s\n", az)
	}

	return &asgTemplate{
		InstanceType: "QCLOUD",
		Region:       config.Region,
		Zone:         az,
		Cpu:cpu,
		Mem:mem,
		Label:asgLabel.Label,
	}, nil
}

func (m *QcloudManager) buildNodeFromTemplate(asg *Asg, template *asgTemplate) (*apiv1.Node, error) {
	node := apiv1.Node{}
	nodeName := fmt.Sprintf("%s-%d", asg.Name, rand.Int63())

	node.ObjectMeta = metav1.ObjectMeta{
		Name:     nodeName,
		SelfLink: fmt.Sprintf("/api/v1/nodes/%s", nodeName),
		Labels:   map[string]string{},
	}

	node.Status = apiv1.NodeStatus{
		Capacity: apiv1.ResourceList{},
	}

	// TODO: get a real value.
	node.Status.Capacity[apiv1.ResourcePods] = *resource.NewQuantity(110, resource.DecimalSI)
	node.Status.Capacity[apiv1.ResourceCPU] = *resource.NewQuantity(int64(template.Cpu), resource.DecimalSI)
	node.Status.Capacity[apiv1.ResourceMemory] = *resource.NewQuantity(int64(template.Mem*1024*1024*1024), resource.DecimalSI)

	// TODO: use proper allocatable!!
	node.Status.Allocatable = node.Status.Capacity

	node.Labels = cloudprovider.JoinStringMaps(node.Labels, template.Label)

	// GenericLabels
	node.Labels = cloudprovider.JoinStringMaps(node.Labels, buildGenericLabels(template, nodeName))

	node.Status.Conditions = cloudprovider.BuildReadyConditions()

	glog.Warningf("buildNodeFromTemplate node:%#v, asg:%s", node, asg.Name)
	return &node, nil
}

func buildGenericLabels(template *asgTemplate, nodeName string) map[string]string {
	result := make(map[string]string)
	// TODO: extract it somehow
	result[kubeletapis.LabelArch] = cloudprovider.DefaultArch
	result[kubeletapis.LabelOS] = cloudprovider.DefaultOS

	result[kubeletapis.LabelInstanceType] = template.InstanceType

	result[kubeletapis.LabelZoneRegion] = template.Region
	result[kubeletapis.LabelZoneFailureDomain] = template.Zone
	result[kubeletapis.LabelHostname] = nodeName
	return result
}
