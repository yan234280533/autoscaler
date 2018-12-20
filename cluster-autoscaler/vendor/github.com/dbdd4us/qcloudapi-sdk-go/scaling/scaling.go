package scaling

import "errors"

type Response struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	CodeDesc string `json:"codeDesc"`
}

type ScalingGroup struct {
	ScalingGroupId         string   `json:"scalingGroupId"`
	ScalingGroupName       string   `json:"scalingGroupName"`
	ScalingConfigurationId string   `json:"scalingConfigurationId"`
	VpcId                  string   `json:"vpcId"`
	SubnetIdSet            []SubnetId `json:"subnetIdSet"`
	InstanceNum            int      `json:"instanceNum"`
	MinSize                int      `json:"minSize"`
	MaxSize                int      `json:"maxSize"`
	DesiredCapacity        int64    `json:"desiredCapacity"`
}

type SubnetId struct {
	SubnetId string   `json:"subnetId"`
	Owner    string   `json:"owner"`
	ZoneId   int      `json:"zoneId"`
	Status   int      `json:"status"`
}

type  ScalingGroupInfo struct {
	ScalingGroupSet []ScalingGroup   `json:"scalingGroupSet"`
	TotalCount      int      `json:"totalCount"`
}

type DescribeScalingGroupArgs struct {
	ScalingGroupIds []string `qcloud_arg:"scalingGroupIds"`
}

type DescribeScalingGroupResponse struct {
	Response
	Data ScalingGroupInfo   `json:"data"`
}

func (client *Client) DescribeScalingGroup(args *DescribeScalingGroupArgs) (*DescribeScalingGroupResponse, error) {
	response := &DescribeScalingGroupResponse{}
	err := client.Invoke("DescribeScalingGroup", args, response)
	if err != nil {
		return &DescribeScalingGroupResponse{}, err
	}
	return response, nil
}

func (client *Client) DescribeScalingGroupById(scalingGroupId string) (*ScalingGroup, error) {
	args := &DescribeScalingGroupArgs{ScalingGroupIds:[]string{scalingGroupId}}
	res, err := client.DescribeScalingGroup(args)
	if err != nil {
		return &ScalingGroup{}, err
	}

	if res.Code != 0 {
		return &ScalingGroup{}, errors.New("DescribeScalingGroup ret code error")
	}

	if res.Data.TotalCount != 1 {
		return &ScalingGroup{}, errors.New("DescribeScalingGroup not found")
	}

	return &res.Data.ScalingGroupSet[0], nil
}

type DescribeScalingConfigurationArgs struct {
	ScalingConfigurationIds []string `qcloud_arg:"scalingConfigurationIds"`
}

type  scalingConfigurationInfo struct {
	ScalingConfigurationSet []ScalingConfiguration   `json:"scalingConfigurationSet"`
	TotalCount              int      `json:"totalCount"`
}

type ScalingConfiguration struct {
	ScalingConfigurationId   string   `json:"scalingConfigurationId"`
	ScalingConfigurationName string   `json:"scalingConfigurationName"`
	Cpu                      int      `json:"cpu"`
	Mem                      int      `json:"mem"`
	Type                     string   `json:"type"`
}

type DescribeScalingConfigurationResponse struct {
	Response
	Data scalingConfigurationInfo   `json:"data"`
}

func (client *Client) DescribeScalingConfiguration(args *DescribeScalingConfigurationArgs) (*DescribeScalingConfigurationResponse, error) {
	response := &DescribeScalingConfigurationResponse{}
	err := client.Invoke("DescribeScalingConfiguration", args, response)
	if err != nil {
		return &DescribeScalingConfigurationResponse{}, err
	}
	return response, nil
}

func (client *Client) DescribeScalingConfigurationById(scalingConfigurationId string) (*ScalingConfiguration, error) {
	args := &DescribeScalingConfigurationArgs{ScalingConfigurationIds:[]string{scalingConfigurationId}}
	res, err := client.DescribeScalingConfiguration(args)
	if err != nil {
		return &ScalingConfiguration{}, err
	}

	if res.Code != 0 {
		return &ScalingConfiguration{}, errors.New("DescribeScalingConfiguration ret code error")
	}

	if res.Data.TotalCount != 1 {
		return &ScalingConfiguration{}, errors.New("DescribeScalingConfiguration not found")
	}

	return &res.Data.ScalingConfigurationSet[0], nil
}


type DescribeScalingActivityReq struct {
	ScalingGroupId string `qcloud_arg:"scalingGroupId"`
	ScalingActivityIds    []string `qcloud_arg:"scalingActivityIds"`
}

type DescribeScalingActivityResponse struct {
	Response
	Data scalingActivityInfo   `json:"data"`
}

type  scalingActivityInfo struct {
	ScalingActivitySet      []scalingActivity   `json:"scalingActivitySet"`
	TotalCount              int      `json:"totalCount"`
}

type scalingActivity struct {
	AutoScalingGroupId   string   `json:"autoScalingGroupId"`
	Status               int      `json:"status"`
}

func (client *Client) DescribeScalingActivity(args *DescribeScalingActivityReq) (*DescribeScalingActivityResponse, error) {
	response := &DescribeScalingActivityResponse{}
	err := client.Invoke("DescribeScalingActivity", args, response)
	if err != nil {
		return &DescribeScalingActivityResponse{}, err
	}
	return response, nil
}

func (client *Client) DescribeScalingActivityById(scalingGroupId, scalingActivityId string) (int, error) {
	args := &DescribeScalingActivityReq{ScalingActivityIds:[]string{scalingActivityId}, ScalingGroupId:scalingGroupId}
	res, err := client.DescribeScalingActivity(args)
	if err != nil {
		return 0, err
	}

	if res.Code != 0 {
		return 0, errors.New("DescribeScalingActivity ret code error")
	}

	if res.Data.TotalCount != 1 {
		return 0, errors.New("DescribeScalingActivity not found")
	}

	return res.Data.ScalingActivitySet[0].Status, nil
}

type DetachInstanceArgs struct {
	ScalingGroupId string   `qcloud_arg:"scalingGroupId"`
	InstanceIds    []string `qcloud_arg:"instanceIds"`
	KeepInstance   int      `qcloud_arg:"keepInstance"`
}

type DetachInstanceResponse struct {
	Response
	Data struct{
		     ScalingActivityId string   `qcloud_arg:"scalingActivityId"`
	     }   `json:"data"`
}

func (client *Client) DetachInstance(args *DetachInstanceArgs) (*DetachInstanceResponse, error) {
	response := &DetachInstanceResponse{}
	err := client.Invoke("DetachInstance", args, response)
	if err != nil {
		return &DetachInstanceResponse{}, err
	}
	if response.Code != 0 {
		return nil, errors.New("DetachInstance ret code error")
	}
	return response, nil
}

type ModifyScalingGroupArgs struct {
	ScalingGroupId  string `qcloud_arg:"scalingGroupId"`
	DesiredCapacity int64    `qcloud_arg:"desiredCapacity"`
}

type ModifyScalingGroupResponse struct {
	Response
}

func (client *Client) ModifyScalingGroup(args *ModifyScalingGroupArgs) (*ModifyScalingGroupResponse, error) {
	response := &ModifyScalingGroupResponse{}
	err := client.Invoke("ModifyScalingGroup", args, response)
	if err != nil {
		return &ModifyScalingGroupResponse{}, err
	}
	return response, nil
}

type DescribeScalingInstanceArgs struct {
	ScalingGroupId string `qcloud_arg:"scalingGroupId"`
	Offset int `qcloud_arg:"offset"`
	Limit  int `qcloud_arg:"limit"`
}

type  ScalingInstancesInfo struct {
	ScalingInstancesSet []ScalingInstance   `json:"scalingInstancesSet"`
	TotalCount          int      `json:"totalCount"`
}

type ScalingInstance struct {
	InstanceId           string   `json:"instanceId"`
	HealthStatus         string   `json:"healthStatus"`
	CreationType         string   `json:"creationType"`
	LifeCycleState       string   `json:"lifeCycleState"`
	ProtectedFromScaleIn int      `json:"protectedFromScaleIn"`
}

type DescribeScalingInstanceResponse struct {
	Response
	Data ScalingInstancesInfo   `json:"data"`
}

func (client *Client) DescribeScalingInstance(args *DescribeScalingInstanceArgs) (*DescribeScalingInstanceResponse, error) {
	response := &DescribeScalingInstanceResponse{}
	err := client.Invoke("DescribeScalingInstance", args, response)
	if err != nil {
		return &DescribeScalingInstanceResponse{}, err
	}
	return response, nil
}

func (client *Client) DescribeScalingInstanceById(scalingGroupId string) (*[]ScalingInstance, error) {
	args := &DescribeScalingInstanceArgs{ScalingGroupId:scalingGroupId}
	res, err := client.DescribeScalingInstance(args)
	if err != nil {
		return nil, err
	}

	if res.Code != 0 {
		return nil, errors.New("DescribeScalingInstance ret code error")
	}

	return &res.Data.ScalingInstancesSet, nil
}

