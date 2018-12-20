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
	"fmt"

	"github.com/golang/glog"
	"cloud.tencent.com/tencent-cloudprovider/cbs-volume/lib/controller"
	"k8s.io/api/core/v1"
)

func (p *cbsProvisioner) Delete(volume *v1.PersistentVolume) error {
	glog.Infof("Delete called for volume:", volume.Name)

	provisioned, err := p.provisioned(volume)
	if err != nil {
		return fmt.Errorf("error determining if this provisioner was the one to provision volume %q: %v", volume.Name, err)
	}
	if !provisioned {
		strerr := fmt.Sprintf("this provisioner id %s didn't provision volume %q and so can't delete it; id %s did & can", p.identity, volume.Name, volume.Annotations[annProvisionerID])
		return &controller.IgnoredError{Reason: strerr}
	}

	err = p.provider.DeleteDisk(volume.Spec.QcloudCbs.CbsDiskId)
	if err != nil {
		glog.Errorf("Failed to delete volume %s,error: %s", volume, err.Error())
		return err
	}
	return nil
}

func (p *cbsProvisioner) provisioned(volume *v1.PersistentVolume) (bool, error) {
	createBy, ok := volume.Annotations[annCreatedBy]
	if !ok {
		return false, fmt.Errorf("PV doesn't have an annotation %s", annCreatedBy)
	}

	return createBy == createdByCbs, nil
}
