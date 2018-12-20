package ccs

import (
	"errors"
	"reflect"
)

type Response struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	CodeDesc string `json:"codeDesc"`
}

type DescribeAsgLabelArgs struct {
	ScalingGroupId string `qcloud_arg:"autoScalingGroupId"`
}

type  AsgLabelSet struct {
	AsgInfo    []AsgLabelInfo   `json:"asgInfo"`
	TotalCount int              `json:"totalCount"`
}

type AsgLabelInfo struct {
	AutoScalingGroupId string   `json:"autoScalingGroupId"`
	ClusterInstanceId  string   `json:"clusterId"`
	LabelTmp           interface{}   `json:"label"`
	Label              map[string]string
}

type DescribeAsgLabelResponse struct {
	Response
	Data AsgLabelSet   `json:"data"`
}

func (client *Client) DescribeAsgLabel(scalingGroupId string) (*AsgLabelInfo, error) {
	args := &DescribeAsgLabelArgs{ScalingGroupId:scalingGroupId}
	response := &DescribeAsgLabelResponse{}
	err := client.Invoke("DescribeClusterAsg", args, response)
	if err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, errors.New("DescribeAsgLabel ret code error")
	}

	if response.Data.TotalCount != 1 {
		return nil, errors.New("DescribeAsgLabel ret count error")
	}

	indirectKind := reflect.Indirect(reflect.ValueOf(response.Data.AsgInfo[0].LabelTmp)).Kind()
	if indirectKind != reflect.Map {
		response.Data.AsgInfo[0].Label = make(map[string]string, 0)
	}else {
		response.Data.AsgInfo[0].Label = make(map[string]string, 0)
		for key, val := range response.Data.AsgInfo[0].LabelTmp.(map[string]interface{}) {
			response.Data.AsgInfo[0].Label[key] = val.(string)
		}
	}

	return &response.Data.AsgInfo[0], nil
}

type DeleteClusterInstancesReq struct {
	ClusterId   string   `qcloud_arg:"clusterId"`
	InstanceIds []string `qcloud_arg:"instanceIds"`
}

type DeleteClusterInstancesResponse struct {
	Response
}

func (client *Client) DeleteClusterInstances(req DeleteClusterInstancesReq) error {
	response := &DeleteClusterInstancesResponse{}
	err := client.Invoke("DeleteClusterInstances", req, response)
	if err != nil {
		return err
	}

	if response.Code != 0 {
		return errors.New("DeleteClusterInstances ret code error")
	}

	return nil
}

