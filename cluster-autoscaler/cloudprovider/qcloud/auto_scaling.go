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
	"errors"
	"fmt"

	autoscaling "github.com/dbdd4us/qcloudapi-sdk-go/scaling"
	"github.com/dbdd4us/qcloudapi-sdk-go/ccs"
	"github.com/golang/glog"
)

// autoScalingWrapper provides several utility methods over the auto-scaling service provided by QCLOUD SDK
type autoScalingWrapper struct {
	Client *autoscaling.Client
	CcsClient *ccs.Client
}

const (
	maxRecordsReturnedByAPI = 100
)

func (m autoScalingWrapper) getInstanceTypeByLCName(name string) (int, int, error) {
	params := &autoscaling.DescribeScalingConfigurationArgs{
		ScalingConfigurationIds:[]string{name},
	}
	launchConfigurations, err := m.Client.DescribeScalingConfiguration(params)
	if err != nil {
		glog.V(4).Infof("Failed LaunchConfiguration info request for %s: %v", name, err)
		return 0, 0, err
	}
	if len(launchConfigurations.Data.ScalingConfigurationSet) < 1 {
		return 0, 0, fmt.Errorf("Unable to get first LaunchConfiguration for %s", name)
	}

	return launchConfigurations.Data.ScalingConfigurationSet[0].Cpu, launchConfigurations.Data.ScalingConfigurationSet[0].Mem, nil
}

func (m autoScalingWrapper) getAutoscalingGroupByName(name string) (*autoscaling.ScalingGroup, error) {
	params := &autoscaling.DescribeScalingGroupArgs{
		ScalingGroupIds: []string{name},
	}
	groups, err := m.Client.DescribeScalingGroup(params)
	if err != nil {
		glog.V(4).Infof("Failed ASG info request for %s: %v", name, err)
		return nil, err
	}
	if len(groups.Data.ScalingGroupSet) < 1 {
		return nil, fmt.Errorf("Unable to get first autoscaling.Group for %s", name)
	}
	return &groups.Data.ScalingGroupSet[0], nil
}

func (m autoScalingWrapper) getAutoscalingGroupLabel(name string) (*ccs.AsgLabelInfo, error) {
	asgLabelInfo, err := m.CcsClient.DescribeAsgLabel(name)
	if err != nil {
		glog.V(4).Infof("Failed DescribeAsgLabel info request for %s: %v", name, err)
		return nil, err
	}

	return asgLabelInfo, nil
}

func (m *autoScalingWrapper) getAutoscalingGroupsByNames(names []string) ([]autoscaling.ScalingGroup, error) {
	glog.V(6).Infof("Starting getAutoscalingGroupsByNames with names=%v", names)

	params := &autoscaling.DescribeScalingGroupArgs{
		ScalingGroupIds: names,
	}
	description, err := m.Client.DescribeScalingGroup(params)
	if err != nil {
		glog.V(4).Infof("Failed to describe ASGs : %v", err)
		return nil, err
	}
	if len(description.Data.ScalingGroupSet) < 1 {
		return nil, errors.New("No ASGs found")
	}

	glog.V(6).Infof("Finishing getAutoscalingGroupsByNames asgs=%v", description.Data.ScalingGroupSet)

	return description.Data.ScalingGroupSet, nil
}


func (m autoScalingWrapper) getAutoscalingInstance(name string) ([]autoscaling.ScalingInstance, error) {
	params := &autoscaling.DescribeScalingInstanceArgs{
		ScalingGroupId: name,
		Offset:0,
		Limit:maxRecordsReturnedByAPI,
	}
	groups, err := m.Client.DescribeScalingInstance(params)
	if err != nil {
		glog.V(4).Infof("Failed ASG info request for %s: %v", name, err)
		return nil, err
	}

	ins := groups.Data.ScalingInstancesSet
	for int(len(ins)) < groups.Data.TotalCount {
		params := &autoscaling.DescribeScalingInstanceArgs{
			ScalingGroupId: name,
			Offset:int(len(ins)),
			Limit:maxRecordsReturnedByAPI,
		}
		groups, err = m.Client.DescribeScalingInstance(params)
		if err != nil {
			glog.V(4).Infof("Failed ASG info request for %s: %v", name, err)
			return nil, err
		}else {
			ins = append(ins, groups.Data.ScalingInstancesSet...)
		}
	}

	return ins, nil
}

