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
	"github.com/dbdd4us/qcloudapi-sdk-go/metadata"
	"github.com/golang/glog"
)

//避免对metadata服务的强依赖
//假设 instanceId和PrivateIP是不变的，故优先从cache中获取
//public优先从metadata中获取

type metaDataCached struct {
	metaData    *metadata.MetaData
	instanceId  string
	privateIPv4 string
	publicIPv4  *string // 可能为nil
}

func newMetaDataCached() *metaDataCached {
	return &metaDataCached{
		metaData: metadata.NewMetaData(nil),
	}
}

func (cached *metaDataCached) InstanceID() (string, error) {
	if cached.instanceId != "" {
		return cached.instanceId, nil
	}
	rsp, err := cached.metaData.InstanceID()
	if err != nil {
		return "", err
	}
	cached.instanceId = rsp
	return cached.instanceId, nil
}

func (cached *metaDataCached) PrivateIPv4() (string, error) {
	if cached.privateIPv4 != "" {
		return cached.privateIPv4, nil
	}
	rsp, err := cached.metaData.PrivateIPv4()
	if err != nil {
		return "", err
	}

	cached.privateIPv4 = rsp
	return cached.privateIPv4, nil
}

//反回 "" 时，公网IP不存在
func (cached *metaDataCached) PublicIPv4() (string, error) {
	rsp, err := cached.metaData.PublicIPv4()
	if err != nil {
		if cached.publicIPv4 == nil {
			return "", err
		}
		glog.Warningf("metaData.PublicIPv4() get err :%s, use cached: %s", err, *cached.publicIPv4)
		return *cached.publicIPv4, nil
	}
	cached.publicIPv4 = &rsp
	return *cached.publicIPv4, nil
}
