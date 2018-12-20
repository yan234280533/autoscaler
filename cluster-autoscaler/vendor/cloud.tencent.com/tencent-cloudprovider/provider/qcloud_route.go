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
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

func (self *QCloud) ListRoutes(clusterName string) ([]*cloudprovider.Route, error) {
	routes := make([]*cloudprovider.Route, 0)
	req := norm.NormListRoutesReq{ClusterName: clusterName}
	rsp, err := norm.NormListRoutes(req)
	if err != nil {
		return nil, err
	}
	for _, item := range rsp.Routes {
		if item.Subnet == "" {
			continue
		}
		routes = append(routes, &cloudprovider.Route{
			Name:            "", // TODO: what's this?
			TargetNode:      types.NodeName(item.Name),
			DestinationCIDR: item.Subnet,
		})
	}
	return routes, nil
}

func (self *QCloud) CreateRoute(clusterName string, nameHint string, route *cloudprovider.Route) error {
	glog.Infof("qcloud create route: %s %s %#v\n", clusterName, nameHint, route)
	req := []norm.NormRouteInfo{
		{Name: string(route.TargetNode), Subnet: route.DestinationCIDR},
	}
	_, err := norm.NormAddRoute(req)
	return err
}

// DeleteRoute deletes the specified managed route
// Route should be as returned by ListRoutes
func (self *QCloud) DeleteRoute(clusterName string, route *cloudprovider.Route) error {

	req := []norm.NormRouteInfo{
		{Name: string(route.TargetNode), Subnet: route.DestinationCIDR},
	}
	_, err := norm.NormDelRoute(req)
	return err
}
