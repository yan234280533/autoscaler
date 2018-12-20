package component

import (
	"testing"
	"fmt"
	"flag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/client-go/1.4/_vendor/github.com/davecgh/go-spew/spew"
)

const (
	UNITTEST_POD_CIDR = "100.64.0.0/16"
	UNITTEST_NODE_NAME = "10.3.2.95"
)

func initGlog() {
	flag.Set("logtostderr", "true")
}

func TestNormGetNodeInfo(t *testing.T) {
	initGlog()
	req := NormGetNodeInfoReq{}
	rsp, err := NormGetNodeInfo(req)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%#v\n", rsp)
}

func xTestNormAddRoute(t *testing.T) {
	initGlog()
	req := []NormRouteInfo{{
		Name: UNITTEST_NODE_NAME,
		Subnet: UNITTEST_POD_CIDR,
	}}
	rsp, err := NormAddRoute(req)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%#v\n", rsp)
}

func TestNormListRoutes(t *testing.T) {
	initGlog()
	req := NormListRoutesReq{
	}
	rsp, err := NormListRoutes(req)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%#v\n", rsp)
}

func xTestNormDelRoute(t *testing.T) {
	initGlog()
	req := []NormRouteInfo{{
		Name: UNITTEST_NODE_NAME,
		Subnet: UNITTEST_POD_CIDR,
	}}
	rsp, err := NormDelRoute(req)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%#v\n", rsp)
}

func TestNormCommonPassLB(t *testing.T) {
	initGlog()

	NormRsp, _ := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.queryLBInfo", QueryLbRequest{
		Offset:0,
		Limit:987,
	})
	fmt.Printf("%#v\n", NormRsp)

}

func TestNormCommonPassLBError(t *testing.T) {
	initGlog()

	_, err := QueryLbCommonPass("abc")
	if err == nil {
		t.Error("except error!")
	}
	fmt.Printf("%#v\n", err)
	return

}

func TestNormGetLbListerInfoError(t *testing.T) {
	_, err := GetLbListerInfo("lb-123")
	if err == nil {
		t.Error("except error!")
	}
	fmt.Printf("%#v\n", err)
	return

}

//func TestCreateLbListerError(t *testing.T) {
//	PortMap := []PortMapInfo{PortMapInfo{Vport:80, Pport:80, Protocol:LB_LISTENER_PROTOCAL_TCP}}
//
//	err := CreateLbLister("lb-123", PortMap)
//	if err == nil {
//		t.Error("except error!")
//	}
//	fmt.Printf("%#v\n", err)
//	return
//
//}

const (
	LB_INFO_RSP = `{"eventId": 3069595155, "returnData": {"passData": {"eventId": 83014, "returnCode": 0, "componentName": "cgateway", "version": 1, "returnValue": 0, "timestamp": 1478698790, "returnMessage": "ok", "data": {"count": 0, "LBList": [{"modTimestamp": "2016-11-09 21:38:11", "orderId": 2618369, "vpcId": 5389, "sessionExpire": 0, "uLBId": "lb-a3thnjt1", "LBType": "open", "LBId": 39791, "loadBalanceId": "lb-gz-39791", "LBName": "garyyu-svc-test", "subnetId": 0, "projectId": 0, "vipInfoList": [{"vip": "119.29.51.24", "ispId": "5"}], "addTimestamp": "2016-11-09 21:26:22", "forward": 0, "status": 1, "desState": 0, "switchFlag": "1", "appId": 1251664966, "LBDomain": "garyyu-svc-test.gz.1251664966.clb.myqcloud.com", "openBgp": false, "deviceIdList": [""], "randId": "dVvUDfiyB2u57bi", "new7switch": true}], "page": null, "totalNum": 1}}}, "version": "1.0", "returnValue": 0, "returnMsg": "success", "timestamp": 1478698790, "caller": "cloudprovider", "callee": "NORM"}`
	LB_LISTEN_INFP_RSP = `{"eventId": 3177527534, "returnData": {"passData": {"eventId": 864872, "returnCode": 0, "componentName": "cgateway", "version": 1, "returnValue": 0, "timestamp": 1478698728, "returnMessage": "ok", "data": [{"modTimestamp": "2016-11-09 21:38:11", "certCaId": "", "protocol": "tcp", "intervalTime": 5, "LBId": 39791, "httpGzip": true, "sessionExpire": 0, "vport": 80, "certId": "", "httpCheckPath": "", "httpCode": 31, "addTimestamp": "2016-11-09 21:38:09", "SSLMode": "", "portMapId": 114347, "status": 1, "healthNum": 3, "healthStatus": 1, "timeOut": 2, "httpHash": "", "unhealthNum": 3, "pport": 30189, "uListenerId": "lbl-d5wcdgzp", "listenerName": "", "healthSwitch": true}]}}, "version": "1.0", "returnValue": 0, "returnMsg": "success", "timestamp": 1478698728, "caller": "cloudprovider", "callee": "NORM"}`
)

func TestUnPackLBSubRequest(t *testing.T) {
	nc := NewNormClient()

	rsp := &NormCallPassRsp{}
	err := nc.unPackResponse([]byte(LB_INFO_RSP), rsp)
	if err != nil {
		t.Error(err)
	}
	t.Log(spew.Sdump(rsp))

	LBRsp := &QueryLbResponse{}
	if err = unPackPassResponse(*rsp, LBRsp); err != nil {
		t.Error(err)
	}
	t.Log(spew.Sdump(LBRsp.LBList[0]))

}

func TestGetLbListerInfo(t *testing.T) {
	nc := NewNormClient()

	rsp := &NormCallPassRsp{}
	err := nc.unPackResponse([]byte(LB_LISTEN_INFP_RSP), rsp)
	if err != nil {
		t.Error(err)
	}
	t.Log(spew.Sdump(rsp))

	code, msg, err := getCodeMsg(*rsp)
	if err != nil {
		t.Error(err)
	}

	t.Logf("code:%d,msg:%s", code, msg)
	LBRsp := make([]*ListerInfo, 0)
	if err = unPackPassResponse(*rsp, &LBRsp); err != nil {
		t.Error(err)
	}
	t.Log(spew.Sdump(LBRsp))

}

func TestDiffListerPort(t *testing.T) {

	apiService := &api.Service{Spec:api.ServiceSpec{Ports:[]api.ServicePort{
		api.ServicePort{Port:80, Protocol:"udp", NodePort:80},
		api.ServicePort{Port:81, Protocol:"tcp", NodePort:8888},
		api.ServicePort{Port:82, Protocol:"tcp", NodePort:82},
		api.ServicePort{Port:88, Protocol:"tcp", NodePort:88},
	}}}

	LBListeners := make([]*ListerInfo, 0)
	LBListeners = append(LBListeners, &ListerInfo{UListenerId:"UListenerId-80", Vport:80, Protocol:"tcp", Pport:80}) // to del UListenerId-80  and to add udp 80:80
	LBListeners = append(LBListeners, &ListerInfo{UListenerId:"UListenerId-81", Vport:81, Protocol:"tcp", Pport:81}) // to del UListenerId-81  and to add tcp 81:8888
	LBListeners = append(LBListeners, &ListerInfo{UListenerId:"UListenerId-85", Vport:85, Protocol:"tcp", Pport:82})   // to del UListenerId-85 and to add tcp 82:82
	LBListeners = append(LBListeners, &ListerInfo{UListenerId:"UListenerId-88", Vport:88, Protocol:"tcp", Pport:88})  //noth

	a, b := DiffListerPort(apiService, LBListeners)
	fmt.Printf("a:%#v \n b:%#v\n", apiService, LBListeners)

	fmt.Printf("a:%#v \n b:%#v\n", a, b)

}


