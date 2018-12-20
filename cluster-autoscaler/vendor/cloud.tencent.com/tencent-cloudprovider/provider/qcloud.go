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
	"github.com/golang/glog"
	"io"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
	//"k8s.io/apimachinery/pkg/types"

	norm "cloud.tencent.com/tencent-cloudprovider/component"
	"cloud.tencent.com/tencent-cloudprovider/credential"

	"github.com/dbdd4us/qcloudapi-sdk-go/cbs"
	"github.com/dbdd4us/qcloudapi-sdk-go/clb"
	"github.com/dbdd4us/qcloudapi-sdk-go/cvm"
	"github.com/dbdd4us/qcloudapi-sdk-go/snap"

	"github.com/dbdd4us/qcloudapi-sdk-go/common"

	"encoding/json"
	"errors"
	"time"
)

const (
	ProviderName                      = "qcloud"
	AnnoServiceLBInternalSubnetID     = "service.kubernetes.io/qcloud-loadbalancer-internal"
	AnnoServiceLBInternalUniqSubnetID = "service.kubernetes.io/qcloud-loadbalancer-internal-subnetid"

	AnnoServiceClusterId = "service.kubernetes.io/qcloud-loadbalancer-clusterid"
)

//TODO instance cache
type QCloud struct {
	currentInstanceInfo *norm.NormGetNodeInfoRsp
	metaData            *metaDataCached
	cvm                 *cvm.Client
	clb                 *clb.Client
	cbs                 *cbs.Client
	snap                *snap.Client

	Config           *Config
	selfInstanceInfo *cvm.InstanceInfo
}

type Config struct {
	Region string `json:"region"`
	Zone   string `json:"zone"`
	VpcId  string `json:"vpcId"`

	QCloudSecretId  string `json:"QCloudSecretId"`
	QCloudSecretKey string `json:"QCloudSecretKey"`

	Kubeconfig string `json:"kubeconfig"`
}

var (
	config                 Config
	QcloudInstanceNotFound = errors.New("qcloud instance not found")
)

func readConfig(cfg io.Reader) error {
	if cfg == nil {
		err := fmt.Errorf("No cloud provider config given")
		return err
	}

	if err := json.NewDecoder(cfg).Decode(&config); err != nil {
		glog.Errorf("Couldn't parse config: %v", err)
		return err
	}
	glog.Info("config:%v", config)

	return nil
}

//TODO if is master
func newQCloud() (*QCloud, error) {

	var cred common.CredentialInterface

	if config.QCloudSecretId == "" || config.QCloudSecretKey == "" {
		expiredDuration := time.Second * 7200

		refresher, err := credential.NewNormRefresher(expiredDuration)
		if err != nil {
			fmt.Println(err)
		}
		normCred, err := credential.NewNormCredential(expiredDuration, refresher)
		if err != nil {
			fmt.Println(err)
		}
		cred = &normCred

	} else {
		cred = common.Credential{
			SecretId:  config.QCloudSecretId,
			SecretKey: config.QCloudSecretKey,
		}
	}

	cvmClient, err := cvm.NewClient(
		cred,
		common.Opts{
			Region: config.Region,
		})
	if err != nil {
		return nil, err
	}
	//cvmClient.SetDebug(true)

	clbClient, err := clb.NewClient(
		cred,
		common.Opts{
			Region: config.Region,
		})
	if err != nil {
		return nil, err
	}

	cbsClient, err := cbs.NewClient(
		cred,
		common.Opts{
			Region: config.Region,
		})
	if err != nil {
		return nil, err
	}

	snapClient, err := snap.NewClient(
		cred,
		common.Opts{
			Region: config.Region,
		})
	if err != nil {
		return nil, err
	}

	cloud := &QCloud{
		Config:   &config,
		metaData: newMetaDataCached(),
		cvm:      cvmClient,
		clb:      clbClient,
		cbs:      cbsClient,
		snap:     snapClient,
	}

	return cloud, nil
}

func retrieveErrorCodeAndMessage(err error) (int, string) {
	if derr, ok := err.(*norm.RequestResultError); ok {
		return derr.Code, derr.Msg
	}
	return -999999, err.Error()
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(cfg io.Reader) (cloudprovider.Interface, error) {
		err := readConfig(cfg)
		if err != nil {
			return nil, err
		}
		return newQCloud()
	})
}

func (cloud *QCloud) Initialize(clientBuilder controller.ControllerClientBuilder) {}

func (cloud *QCloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return cloud, true
}

func (cloud *QCloud) Instances() (cloudprovider.Instances, bool) {
	return cloud, true
}

func (cloud *QCloud) Zones() (cloudprovider.Zones, bool) {
	return cloud, true
}

func (cloud *QCloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (cloud *QCloud) Routes() (cloudprovider.Routes, bool) {
	return cloud, true
}

func (cloud *QCloud) ProviderName() string {
	return ProviderName
}

func (cloud *QCloud) ScrubDNS(nameservers, seraches []string) ([]string, []string) {
	return nameservers, seraches
}
