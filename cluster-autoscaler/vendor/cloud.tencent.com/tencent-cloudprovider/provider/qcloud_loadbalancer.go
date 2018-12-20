/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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
	norm "cloud.tencent.com/tencent-cloudprovider/component"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"strconv"
	"strings"
	"time"
)

const (
	QUERY_LIMIT       = 10
	QUERY_PERIOD      = 3
	DONE_MAGIC_NUM    = 5
	STATUS_TASK_SUCC  = 0
	STATUS_TASK_FAIL  = 1
	STATUS_TASK_GOING = 2

	LB_CREATE_TYPE_INTERNAL = "internal"
	LB_CREATE_TYPE_OPEN     = "open"
)

//有订单还未发货,发货中以及LB正常状态都应该返回exists;没有订单且LB不存在时才返回notxists
//service type修改为非loadbalancer时候,需要删除lb,此时会先调用此接口确认是否已创建成功,是否需要删除
func (self *QCloud) GetLoadBalancer(clusterName string, apiService *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {

	var queryRsp *norm.LBInfo
	queryRsp, exists, err = norm.GetLbInfo(string(apiService.GetUID()))
	if err != nil || queryRsp == nil {
		//如果出错了,或者LB还没创建成功
		return nil, exists, err
	}

	Ingress := []v1.LoadBalancerIngress{}

	for _, v := range queryRsp.VipInfoList {
		Ingress = append(Ingress, v1.LoadBalancerIngress{IP: v.Vip})
	}

	status = &v1.LoadBalancerStatus{Ingress: Ingress}

	return status, true, nil

}

func isTaskDone(tskId int64) bool {
	//poll状态，30秒内还没成功就有问题
	for i := 0; i < QUERY_LIMIT; i++ {
		s, e := norm.QueryTaskResult(tskId)
		if e != nil {
			msg := fmt.Sprintf("QueryTaskResult failed: %s", e.Error())
			glog.Error(msg)
			break
		}
		glog.Infof("TaskID :%v TaskStatus %v", tskId, s)
		switch s {
		case STATUS_TASK_SUCC:
			return true
		case STATUS_TASK_FAIL:
			return false
		case STATUS_TASK_GOING:
		default:
			glog.Error("Unknown Status")
		}
		time.Sleep(time.Second * QUERY_PERIOD)
	}
	return false
}

func (self *QCloud) ensureLBLister(clusterName string, apiService *v1.Service, hosts []string) error {

	//查一查LB状态
	queryRsp, exists, err := checkLBInf(apiService)
	if err != nil || queryRsp == nil || !exists {
		if err != nil {
			return err
		}
		return errors.New(fmt.Sprintf("LB not created...flow into creating LBLister"))
	}

	ULBId := queryRsp.ULBId
	//查询当前LB状态
	listerInfos, err := norm.GetLbListerInfo(ULBId)
	if err != nil {
		return err
	}

	//将当前LB状态和修改LB请求对比
	toDelete, toAdd := norm.DiffListerPort(apiService, listerInfos)

	glog.Info("toDetle listen:", toDelete)
	glog.Info("toAdd listen:", toAdd)

	if len(toDelete) > 0 {
		//需要删除LB
		tskId, err := norm.DeleteLbLister(ULBId, toDelete)
		if err != nil {
			msg := fmt.Sprintf("DeleteLbLister(%s,%#v)  failed!err:%s", ULBId, toDelete, err.Error())
			glog.Errorf(msg)
			return errors.New(msg)
		}
		msg := fmt.Sprintf("DeleteLbLister(%s,%#v) need a retry!", ULBId, toDelete)
		glog.Infof(msg)

		glog.Infof("checking DeleteLbLister")
		if !isTaskDone(tskId) {
			return errors.New(msg)
		}
	}

	if len(toAdd) > 0 {
		//需要新增LB
		tskId, err := norm.CreateLbLister(ULBId, toAdd)
		if err != nil {
			msg := fmt.Sprintf("CreateLbLister(%s,%#v)  failed!err:%s", ULBId, toAdd, err.Error())
			glog.Errorf(msg)
			return errors.New(msg)
		}
		msg := fmt.Sprintf("CreateLbLister(%s,%#v) need a retry!", ULBId, toAdd)
		glog.Infof(msg)

		glog.Infof("checking CreateLbLister")
		if !isTaskDone(tskId) {
			return errors.New(msg)
		}
	}
	return nil
}

func getSubnetIdInt(subnetId string) (int, error) {
	ret, err := strconv.Atoi(subnetId)
	if err != nil {
		return 0, errors.New("invalid format of subnetid")
	}
	return ret, nil
}

func getLbPrefixName(lbName string) (pName string) {
	//TODO 将"_"替换为"-" , 限制最长长度
	pName = strings.Replace(lbName, "_", "-", -1)
	if len(pName) > 20 {
		pName = pName[0:19]
	}
	return pName
}

func (self *QCloud) ensureLBCreate(clusterName string, apiService *v1.Service, hosts []string) error {

	//查一查LB状态
	queryRsp, exists, err := checkLBInf(apiService)
	if err != nil {
		return err
	}

	annotations := apiService.ObjectMeta.Annotations
	uniqSubnetId, hasUniqSubnetId := annotations[AnnoServiceLBInternalUniqSubnetID]
	iSubnetId, hasSubnetId := annotations[AnnoServiceLBInternalSubnetID]
	var subnetId int
	if hasSubnetId && len(iSubnetId) > 0 {
		subnetId, err = getSubnetIdInt(iSubnetId)
		if err != nil {
			return err
		}
	}

	//如果现在有的lb和要创建的lb类型不同，就删除现有的lb然后创建新的lb
	toCreateLBType := LB_CREATE_TYPE_OPEN
	if (hasUniqSubnetId && len(uniqSubnetId) > 0) || (hasSubnetId && len(iSubnetId) > 0) {
		toCreateLBType = LB_CREATE_TYPE_INTERNAL
	}

	//LB存在的话直接退出进行下一步
	if exists && queryRsp != nil && toCreateLBType == queryRsp.LBType {
		return nil
	}

	if queryRsp != nil && queryRsp.LBType != toCreateLBType {
		glog.Info("Switching LB type...")
		if err := self.EnsureLoadBalancerDeleted(clusterName, apiService); err != nil {
			return err
		}
	}

	//create
	lbName := apiService.Name
	//regularize lb name: ccs_[cluster_id]_[service_name]
	lbName = fmt.Sprintf("%s_%s", clusterName, lbName)

	if len(lbName) > 50 {
		lbName = lbName[0:49]
	}

	lbDomainPrefix := getLbPrefixName(lbName)

	if toCreateLBType == LB_CREATE_TYPE_INTERNAL {
		if hasUniqSubnetId {
			err = norm.NormCreateLB(apiService.Namespace, apiService.Name, string(apiService.GetUID()), lbName,
				LB_CREATE_TYPE_INTERNAL, lbDomainPrefix, nil, &uniqSubnetId, apiService.Spec.LoadBalancerIP)
		} else if hasSubnetId {
			err = norm.NormCreateLB(apiService.Namespace, apiService.Name, string(apiService.GetUID()), lbName,
				LB_CREATE_TYPE_INTERNAL, lbDomainPrefix, &subnetId, nil, apiService.Spec.LoadBalancerIP)
		}
	} else {
		if apiService.Spec.LoadBalancerIP != "" {
			glog.Warning("Public IP cannot be specified for LB")
		}

		//default is the open LB
		err = norm.NormCreateLB(apiService.Namespace, apiService.Name, string(apiService.GetUID()), lbName,
			LB_CREATE_TYPE_OPEN, lbDomainPrefix, nil, nil, "")
	}

	if err != nil {
		msg := fmt.Sprintf("create LB request for %s.%s(%s) failed!", apiService.Namespace, apiService.Name, string(apiService.GetUID()))
		glog.Errorf(msg)
		return errors.New(msg)
	}
	//轮询 LB的创建必须用 GetLbInfo来查询
	glog.Infof("checking NormCreateLB")
	for i := 0; i < QUERY_LIMIT; i++ {
		queryRsp, exists, err := norm.GetLbInfo(string(apiService.GetUID()))
		if err == nil && exists && queryRsp != nil {
			//如果创建成功了直接返回
			return nil
		}
		time.Sleep(time.Second * QUERY_PERIOD)
	}

	//轮询多次不成功则重试
	msg := fmt.Sprintf("create LB request for %s.%s(%s) !! time_out trigger retry", apiService.Namespace, apiService.Name, string(apiService.GetUID()))
	glog.Infof(msg)
	return errors.New(msg)
}

func checkLBInf(apiService *v1.Service) (queryRsp *norm.LBInfo, exists bool, err error) {
	for i := 0; i < QUERY_LIMIT; i++ {
		queryRsp, exists, err = norm.GetLbInfo(string(apiService.GetUID()))
		if err != nil {
			msg := fmt.Sprintf("GetLbInfo(%s,%s) LB err!!err:%s", apiService.Namespace, apiService.Name, err.Error())
			glog.Error(msg)
		} else {
			break
		}

		time.Sleep(time.Millisecond * 300)
	}
	return queryRsp, exists, err
}

func getSuccStatus(queryRsp *norm.LBInfo) *v1.LoadBalancerStatus {
	if queryRsp == nil {
		glog.Error("getSuccStatus Error queryRsp is nil")
		return nil
	}
	//生成设置成功的status
	Ingress := []v1.LoadBalancerIngress{}
	for _, v := range queryRsp.VipInfoList {
		Ingress = append(Ingress, v1.LoadBalancerIngress{IP: v.Vip})
	}
	return &v1.LoadBalancerStatus{Ingress: Ingress}
}

//创建LB 绑定Lister 绑定host
func (self *QCloud) ensureLoadBalancer(clusterName string, apiService *v1.Service, hosts []*v1.Node) (*v1.LoadBalancerStatus, error) {

	err := self.ensureLBCreate(clusterName, apiService, nodesToStrings(hosts))
	if err != nil {
		glog.Errorf("ensureLBCreate failed : %v", err.Error())
		return nil, err
	}
	err = self.ensureLBLister(clusterName, apiService, nodesToStrings(hosts))
	if err != nil {
		glog.Errorf("ensureLBLister failed : %v", err.Error())
		return nil, err
	}

	err = self.UpdateLoadBalancer(clusterName, apiService, hosts)
	if err != nil {
		glog.Errorf("ensureLbHost failed : %v", err.Error())
		return nil, err
	}

	//查一查LB状态
	queryRsp, exists, err := checkLBInf(apiService)
	if err != nil || queryRsp == nil || !exists {
		return nil, errors.New(fmt.Sprintf("LB still creating... retry.."))
	}
	return getSuccStatus(queryRsp), nil
}

func nodesToStrings(hosts []*v1.Node) []string {
	var hs []string
	for _, h := range hosts {
		hs = append(hs, h.Name)
	}
	return hs
}

//一次只处理一个:创建LB,新增、删除监听器调用此接口
//对已经创建的Lb的service也可能会调用此接口需要自己检查校验
//如果返回失败k8s会过一段时间重试5,10,20,40,80
func (self *QCloud) EnsureLoadBalancer(clusterName string, apiService *v1.Service, hosts []*v1.Node) (*v1.LoadBalancerStatus, error) {
	glog.V(0).Infof("EnsureLoadBalancer")
	//glog.V(0).Infof("garyyu-test  EnsureLoadBalancer(%s,%s,%s,%s,%s,%s,%s)",
	//	clusterName, spew.Sdump(apiService), spew.Sdump(apiService.TypeMeta), spew.Sdump(apiService.ObjectMeta), spew.Sdump(apiService.Spec),
	//	spew.Sdump(apiService.Status), hosts)

	if apiService.Spec.SessionAffinity != v1.ServiceAffinityNone {
		//TODO 4层会话保持在nodePort的情况下没有意义
		return nil, fmt.Errorf("Unsupported load balancer affinity: %v", apiService.Spec.SessionAffinity)
	}

	if len(apiService.Spec.Ports) == 0 {
		return nil, fmt.Errorf("requested load balancer with no ports")
	}

	s, e := self.ensureLoadBalancer(clusterName, apiService, hosts)
	if e != nil {
		glog.V(0).Infof("ensureLoadBalancer error :%v", e.Error())
	} else {
		glog.V(0).Info("Create LB Completed!")
	}
	return s, e
}

//新增node调用此接口,删除node不调用此接口
//此接口可能会重复调用,需要确认lb侧是否是可重入的
func (self *QCloud) UpdateLoadBalancer(clusterName string, apiService *v1.Service, hosts []*v1.Node) error {
	glog.V(0).Infof("garyyu-test  UpdateLoadBalancer")

	needChange, err := self.updateLBHost(apiService, nodesToStrings(hosts))
	if err != nil {
		return err
	}
	if !needChange {
		return nil
	}

	return errors.New("modify Host,trigger retry")
}

func (self *QCloud) instanceUuid(lanIp string) (string, error) {
	if rsp, err := norm.NormGetNodeInfoByName(lanIp); err != nil {
		code, _ := retrieveErrorCodeAndMessage(err)
		glog.Info(code)
		if code == -8017 {
			return "", cloudprovider.InstanceNotFound
		}
		return "", err
	} else {
		return rsp.Uuid, nil
	}
}

func (self *QCloud) nodeNamesToUuid(nodeName []string) ([]string, error) {
	var uuids []string
	for _, lanIp := range nodeName {
		uuid, err := self.instanceUuid(lanIp)
		if err != nil {
			if err == cloudprovider.InstanceNotFound {
				glog.Infof("nodeNamesToUuid:not found node(%s),continue", lanIp)
				continue
			}
			return nil, err
		}
		uuids = append(uuids, uuid)
	}
	return uuids, nil
}

func (self *QCloud) checkAddOrDel(ULBId string, hosts []string) (toDelete []string, toAdd []string, needChange bool, err error) {

	uuidsInK8s, err := self.nodeNamesToUuid(hosts)
	if err != nil {
		msg := fmt.Sprintf("nodeNamesToUuid(%#v)  failed!err:%s", hosts, err.Error())
		glog.Errorf(msg)
		return nil, nil, true, errors.New(msg)
	}
	msg := fmt.Sprintf("nodeNamesToUuid(%#v) request succ!", hosts)
	glog.Infof(msg)

	uuidsInLB, err := norm.GetLBHost(ULBId)
	if err != nil {
		msg := fmt.Sprintf("GetLBHost(%#v)  failed!err:%s", ULBId, err.Error())
		glog.Errorf(msg)
		return nil, nil, true, errors.New(msg)
	}
	msg = fmt.Sprintf("GetLBHost(%#v) request succ!", uuidsInLB)
	glog.Infof(msg)

	toDelete, toAdd = norm.InstancesDiff(uuidsInLB, uuidsInK8s)
	return toDelete, toAdd, false, nil
}

func (self *QCloud) updateLBHost(apiService *v1.Service, hosts []string) (needChange bool, err error) {

	var queryRsp *norm.LBInfo
	queryRsp, exists, err := norm.GetLbInfo(string(apiService.GetUID()))
	if err != nil {
		msg := fmt.Sprintf("GetLbInfo(%s,%s) LB err!!err:%s", apiService.Namespace, apiService.Name, err.Error())
		glog.Error(msg)
		return true, errors.New(msg)
	}

	if !exists {
		msg := fmt.Sprintf("UpdateLoadBalancer(%s,%s) LB err!!LB not exists!", apiService.Namespace, apiService.Name)
		glog.Warning(msg)
		return true, errors.New(msg)
	}
	if queryRsp == nil {
		msg := fmt.Sprintf("UpdateLoadBalancer(%s,%s) LB err!!LB not created succ yet!wait", apiService.Namespace, apiService.Name)
		glog.Warning(msg)
		return true, errors.New(msg)
	}

	//=======================================================================================
	ULBId := queryRsp.ULBId
	toDelete, toAdd, needChg, err := self.checkAddOrDel(ULBId, hosts)
	if err != nil {
		return needChg, err
	}
	//=======================================================================================

	glog.Info("toDetle host:", toDelete)
	glog.Info("toAdd host:", toAdd)

	if len(toDelete) == 0 && len(toAdd) == 0 {
		return false, nil
	}
	var msg string
	if len(toDelete) > 0 {
		tskId, err := norm.UnBindCvmToLb(ULBId, toDelete)
		if err != nil {
			msg := fmt.Sprintf("UnBindCvmToLb(%s,%#v)  failed!err:%s", ULBId, toDelete, err.Error())
			glog.Errorf(msg)
			return true, errors.New(msg)
		}
		msg = fmt.Sprintf("UnBindCvmToLb(%s,%#v) request succ!", ULBId, toDelete)
		glog.Infof(msg)

		glog.Infof("checking UnBindCvmToLb")
		//TODO polling task state
		if !isTaskDone(tskId) {
			return false, errors.New(msg)
		}

	}

	if len(toAdd) > 0 {
		tskId, err := norm.BindCvmToLb(ULBId, toAdd)
		if err != nil {
			msg := fmt.Sprintf("BindCvmToLb(%s,%#v)  failed!err:%s", ULBId, toAdd, err.Error())
			glog.Errorf(msg)
			return true, errors.New(msg)
		}
		msg = fmt.Sprintf("BindCvmToLb(%s,%#v) request succ!", ULBId, toAdd)
		glog.Infof(msg)

		glog.Infof("checking BindCvmToLb")
		//TODO polling task state
		if !isTaskDone(tskId) {
			return false, errors.New(msg)
		}
	}

	return false, nil
}

//删除service和修改service类型不为loadbalancer时,会调用此接口
func (self *QCloud) EnsureLoadBalancerDeleted(clusterName string, apiService *v1.Service) error {
	glog.V(0).Infof("garyyu-test  EnsureLoadBalancerDeleted")

	var queryRsp *norm.LBInfo
	queryRsp, exists, err := norm.GetLbInfo(string(apiService.GetUID()))
	if err != nil {
		msg := fmt.Sprintf("GetLbInfo(%s,%s) LB err!!err:%s", apiService.Namespace, apiService.Name, err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	if exists {
		if queryRsp == nil {
			msg := fmt.Sprintf("EnsureLoadBalancerDeleted(%s,%s) LB err!!LB not created succ yet!wait", apiService.Namespace, apiService.Name)
			glog.Warning(msg)
			return errors.New(msg)
		}
		ULBId := queryRsp.ULBId

		tskId, err := norm.DeleteLB(ULBId)
		//polling...
		if err != nil || !isTaskDone(tskId) {
			msg := fmt.Sprintf("DeleteLB(%s) from LB failed!err:%s", ULBId, func(e error) string {
				if e != nil {
					return e.Error()
				}
				return "queryTaskFailed"
			}(err))
			glog.Errorf(msg)
			return errors.New(msg)
		}
		err = norm.NormDeleteLB(string(apiService.GetUID()))
		if err != nil {
			msg := fmt.Sprintf("DeleteLB(%s,%s) from NORM failed!err:%s", ULBId, string(apiService.GetUID()), err.Error())
			glog.Errorf(msg)
			return errors.New(msg)
		}
	} else {
		//如果不存在直接返回成功
		msg := fmt.Sprintf("EnsureLoadBalancerDeleted(%s,%s) LB err!!LB not exists!", apiService.Namespace, apiService.Name)
		glog.Warning(msg)
		return nil
	}

	return nil
}
