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
	"fmt"
	"github.com/dbdd4us/qcloudapi-sdk-go/cbs"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/types"
)

type Disks interface {
	// AttachDisk attaches given disk to given node. Current node
	// is used when nodeName is empty string.
	AttachDisk(diskId string, nodename types.NodeName) (err error)

	// DetachDisk detaches given disk to given node. Current node
	// is used when nodeName is empty string.
	// Assumption: If node doesn't exist, disk is already detached from node.
	DetachDisk(diskId string, nodename types.NodeName) error

	// DiskIsAttached checks if a disk is attached to the given node.
	// Assumption: If node doesn't exist, disk is not attached to the node.
	DiskIsAttached(diskId string, nodename types.NodeName) (bool, error)

	// DisksAreAttached checks if a list disks are attached to the given node.
	// Assumption: If node doesn't exist, disks are not attached to the node.
	DisksAreAttached(diskIds []string, nodename types.NodeName) (map[string]bool, error)

	// CreateDisk creates a new cbs disk with specified parameters.
	CreateDisk(name string, size int, zone string) (diskId string, diskSize int, err error)

	// DeleteVolume deletes cbs disk.
	DeleteDisk(diskId string) error

	BindDiskToAsp(diskId string, asp string) error
}

func (qcloud *QCloud) CreateDisk(diskType string, sizeGb int, zone string) (string, int, error) {

	glog.Infof("CreateDisk, diskType:%s, size:%d, zone:%d",
		diskType, sizeGb, zone)

	var createType string
	var size int

	switch diskType {
	case cbs.StorageTypeCloudBasic, cbs.StorageTypeCloudPremium, cbs.StorageTypeCloudSSD:
		createType = diskType
	default:
		createType = cbs.StorageTypeCloudBasic
	}

	if sizeGb == sizeGb/10*10 {
		size = sizeGb
	} else {
		size = ((sizeGb / 10) + 1) * 10
	}

	if size > 4000 {
		size = 4000
	}

	if createType == cbs.StorageTypeCloudPremium && size < 50 {
		size = 50
	}

	if createType == cbs.StorageTypeCloudSSD && size < 200 {
		size = 200
	}

	args := &cbs.CreateCbsStorageArgs{
		StorageType: createType,
		StorageSize: size,
		PayMode:     cbs.PayModePrePay,
		Period:      1,
		GoodsNum:    1,
		Zone:        zone,
	}

	storageIds, err := qcloud.cbs.CreateCbsStorageTask(args)
	if err != nil {
		return "", 0, err
	}
	if len(storageIds) != 1 {
		return "", 0, fmt.Errorf("err:len(storageIds)=%d", len(storageIds))
	}

	return storageIds[0], size, nil

}

func (qcloud *QCloud) ModifyDiskName(diskId string, name string) error {

	_, err := qcloud.cbs.ModifyCbsStorageAttribute(diskId, name)
	if err != nil {
		return err
	}
	return nil

}

func (qcloud *QCloud) BindDiskToAsp(diskId string, aspId string) error {

	_, err := qcloud.snap.BindAutoSnapshotPolicy(aspId, []string{diskId})
	if err != nil {
		return err
	}
	return nil

}

func (qcloud *QCloud) GetDiskInfo(diskId string) (*cbs.StorageSet, error) {

	rsp, err := qcloud.cbs.DescribeCbsStorage(&cbs.DescribeCbsStorageArgs{
		StorageIds: &[]string{diskId},
	})
	if err != nil {
		glog.Errorf("DescribeCbsStorage failed,diskId:%s,error:%v", diskId, err)
		return nil, err
	}
	if len(rsp.StorageSet) > 1 {
		msg := fmt.Sprintf("DescribeCbsStorage failed,diskId:%s,count:%d > 1", diskId, len(rsp.StorageSet))
		glog.Error(msg)
		return nil, fmt.Errorf(msg)
	}
	if len(rsp.StorageSet) == 0 {
		return nil, nil
	}
	return &rsp.StorageSet[0], nil

}

func (qcloud *QCloud) DeleteDisk(diskId string) error {

	_, err := qcloud.cbs.TerminateCbsStorage([]string{diskId})
	return err

}

func (qcloud *QCloud) AttachDisk(diskId string, nodename types.NodeName) error {

	nodeName := string(nodename)

	glog.Infof("AttachDisk, diskId:%s, node:%s", diskId, nodeName)

	info, err := qcloud.getInstanceInfoByNodeName(nodeName)
	if err != nil || info == nil {
		return fmt.Errorf("AttachCbsDisk failed, disk:%s, get instance:%s, error:%v", diskId, nodeName, err)
	}

	err = qcloud.cbs.AttachCbsStorageTask(diskId, info.InstanceID)
	if err != nil {
		return fmt.Errorf("AttachCbsDisk failed, disk:%s, instance:%s, error:%v", diskId, info.InstanceID, err)
	}

	_, err = qcloud.cbs.ModifyCbsRenewFlag([]string{diskId}, cbs.RenewFlagAutoRenew)
	if err != nil {
		glog.Errorf("ModifyCbsRenewFlag failed, disk:%s, error:%v", diskId, err)
	}

	return nil
}

func (qcloud *QCloud) DiskIsAttached(diskId string, nodename types.NodeName) (bool, error) {
	nodeName := string(nodename)

	glog.Infof("DiskIsAttached, diskId:%s, nodeName:%s", diskId, nodeName)

	instanceInfo, err := qcloud.getInstanceInfoByNodeName(nodeName)
	if err != nil {
		if err == QcloudInstanceNotFound {
			// If instance no longer exists, safe to assume volume is not attached.
			glog.Warningf(
				"Instance %q does not exist. DiskIsAttached will assume disk %q is not attached to it.",
				nodeName,
				diskId)
			return false, nil
		}
		return false, fmt.Errorf("AttachCbsDisk failed, disk:%s, get instance:%s, error:%v", diskId, nodeName, err)
	}
	for _, disk := range instanceInfo.DataDisks {
		if diskId == disk.DiskID {
			return true, nil
		}
	}
	return false, nil
}

func (qcloud *QCloud) DisksAreAttached(diskIds []string, nodename types.NodeName) (map[string]bool, error) {
	nodeName := string(nodename)

	attached := make(map[string]bool)
	for _, diskId := range diskIds {
		attached[diskId] = false
	}

	instanceInfo, err := qcloud.getInstanceInfoByNodeName(nodeName)
	if err != nil {
		if err == QcloudInstanceNotFound {
			glog.Warningf("DiskAreAttached, node is not found, assume disks are not attched to the node:%s",
				nodeName)
			return attached, nil
		}
		return attached, fmt.Errorf("Check DisksAreAttached, get node info failed, disks:%v, get instance:%s, error:%v",
			diskIds, nodeName, err)
	}

	for _, diskId := range diskIds {
		for _, instanceDisk := range instanceInfo.DataDisks {
			if instanceDisk.DiskID == diskId {
				attached[diskId] = true
			}
		}
	}

	return attached, nil
}

func (qcloud *QCloud) DetachDisk(diskId string, nodename types.NodeName) error {
	nodeName := string(nodename)

	glog.Infof("DetachDisk, disk:%s, instance:%s", diskId, nodeName)
	err := qcloud.cbs.DetachCbsStorageTask(diskId)
	if err != nil {
		return fmt.Errorf("DetachCbsStorage failed, disk:%s, error:%s", diskId, err)
	}

	return nil
}
