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
	"k8s.io/autoscaler/cluster-autoscaler/utils/gpu"
	"k8s.io/kubernetes/pkg/kubelet/kubeletconfig/util/log"
	"os"
	"time"

	"cloud.tencent.com/tencent-cloudprovider/credential"
	"encoding/json"
	"github.com/dbdd4us/qcloudapi-sdk-go/ccs"
	"github.com/dbdd4us/qcloudapi-sdk-go/common"
	autoscaling "github.com/dbdd4us/qcloudapi-sdk-go/scaling"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/go-ini/ini"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
	"math/rand"
)

const (
	retryCountDetach   = 2
	intervalTimeDetach = 5 * time.Second
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
	Region    string `json:"region"`
	Zone      string `json:"zone"`
	ClusterId string
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

	//在metacluster托管集群里面没有/etc/kubernetes/config这个配置文件
	//需要从CLUSTER_ID这个环境变量读取clusterId
	//在独立集群里面原始版本为从/etc/kubernetes/config读取ClusterId
	//但独立集群里面错误的把CLUSTER_ID这个环境变量设置成了kube-system
	//所以这里排除clusterIdEnv为空的情况，也要排除clusterIdEnv为kube-system的情况
	//最终逐步切换到所有集群都从CLUSTER_ID这个环境变量读取clusterId信息
	clusterIdEnv := os.Getenv("CLUSTER_ID")
	if (clusterIdEnv != "") && (clusterIdEnv != "kube-system") {
		config.ClusterId = clusterIdEnv
		glog.Infof("read clusterId from env ,clusterId : %s", config.ClusterId)
	} else {
		clsinfo, err := ini.Load("/etc/kubernetes/config")
		if err != nil {
			glog.Errorf("read clusterId from /etc/kubernetes/config failed, %s", err.Error())
			return err
		}

		section := clsinfo.Section("")
		if !section.Haskey("KUBE_CLUSTER") {
			return fmt.Errorf("KUBE_CLUSTER not found")
		}
		config.ClusterId = section.Key("KUBE_CLUSTER").String()
		glog.Infof("read clusterId from /etc/kubernetes/config ,clusterId : %s", config.ClusterId)
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

	normCredential, err := credential.NewNormCredential(time.Second*3600, refresher)
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
		Client:    cli,
		CcsClient: ccsCli,
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
		ScalingGroupId:  asg.Name,
		DesiredCapacity: size,
	}
	glog.V(0).Infof("Setting asg %s size to %d", asg.Id(), size)
	_, err := m.service.Client.ModifyScalingGroup(params)
	if err != nil {
		return err
	}
	return nil
}

func (m *QcloudManager) Cleanup() {
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
		ScalingGroupId: commonAsg.Name,
		InstanceIds:    ins,
		KeepInstance:   1,
	}

	var errOut error
	var scalingActivityId string
	for i := 0; i < retryCountDetach; i++ {
		//从第二次开始，等待5s钟（一般autoscaling移出节点的时间为3s）
		if i > 0 {
			time.Sleep(intervalTimeDetach)
		}
		resp, err := m.service.Client.DetachInstance(params)
		errOut = err

		if err != nil {
			continue
		} else {
			glog.V(4).Infof("res:%#v", resp.Response)
			scalingActivityId = resp.Data.ScalingActivityId
			break
		}
	}

	if errOut != nil {
		return errOut
	}

	//check activity
	err = m.EnsureAS(commonAsg.Name, scalingActivityId)
	if err != nil {
		return err
	}

	//ccs delete node
	delNodePara := ccs.DeleteClusterInstancesReq{ClusterId: config.ClusterId, InstanceIds: ins}
	err = m.service.CcsClient.DeleteClusterInstances(delNodePara)
	if err != nil {
		return err
	}

	return nil
}

func (m *QcloudManager) EnsureAS(scalingGroupId, scalingActivityId string) error {
	if scalingActivityId == "" {
		return nil
	}
	checker := func(r interface{}, e error) bool {
		if e != nil {
			return false
		}
		if r.(int) == 0 {
			return false
		}
		if r.(int) == 1 {
			return false
		}
		return true
	}
	do := func() (interface{}, error) {
		return m.service.Client.DescribeScalingActivityById(scalingGroupId, scalingActivityId)
	}

	status, err, isTimeout := RetryDo(do, checker, 1200, 2)
	if err != nil {
		return fmt.Errorf("EnsureAS get scalingActivityId:%s failed:%v", scalingActivityId, err)
	}

	if isTimeout {
		return fmt.Errorf("EnsureAS scalingActivityId:%s timeout", scalingActivityId)
	}

	if status.(int) != 2 {
		return fmt.Errorf("EnsureAS scalingActivityId:%s fail", scalingActivityId)
	}

	return nil
}

func RetryDo(op func() (interface{}, error), checker func(interface{}, error) bool, timeout uint64, interval uint64) (ret interface{}, err error, isTimeout bool) {
	isTimeout = false
	var tm <-chan time.Time
	tm = time.After(time.Duration(timeout) * time.Second)

	times := 0
	for {
		times = times + 1
		select {
		case <-tm:
			isTimeout = true
			return
		default:
		}
		ret, err = op()
		if checker(ret, err) {
			return
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return
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
	Gpu          int
	Label        map[string]string
}

func (m *QcloudManager) getAsgTemplate(name string) (*asgTemplate, error) {
	asg, err := m.service.getAutoscalingGroupByName(name)
	if err != nil {
		return nil, err
	}

	cpu, mem, gpu, err := m.service.getInstanceTypeByLCName(asg.ScalingConfigurationId)
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
		Cpu:          cpu,
		Mem:          mem,
		Gpu:          gpu,
		Label:        asgLabel.Label,
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

	if template.Gpu > 0 {
		node.Status.Capacity[gpu.ResourceNvidiaGPU] = *resource.NewQuantity(int64(template.Gpu), resource.DecimalSI)
		log.Infof("Capacity resource set gpu %s(%d)", gpu.ResourceNvidiaGPU, template.Gpu)
	}

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
