package component

import (
	"encoding/json"
	"errors"
	"fmt"
	simplejson "github.com/bitly/go-simplejson"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"
	"math/rand"
	"os"
	"reflect"
	"time"
	_ "golang.org/x/text/unicode/norm"
)

const (
	NORM_ADD_NODE = "NORM.AddNode"
	NORM_DEL_NODE = "NORM.DelNode"
	NORM_GET_NODE_INFO = "NORM.GetNodeInfo"
	NORM_GET_NODE_INFO_BY_NAME = "NORM.GetNodeInfoEx"
	NORM_LIST_ROUTES = "NORM.GetRoute"
	NORM_ADD_ROUTE = "NORM.AddRoute"
	NORM_DEL_ROUTE = "NORM.DelRoute"
	NORM_GET_AGENT_CREDENTIAL = "NORM.GetAgentCredential"

	NORM_CREATE_LB = "NORM.CreateLB"
	NORM_DELETE_LB = "NORM.DelLB"
	NORM_GET_LB = "NORM.GetLB"

	NORM_CALL_PASS_MODULE_LB = "NORM.CommonPassLB"
	NORM_CALL_PASS_MODULE_CBS = "NORM.CommonPassCBS"

	NORM_CREATE_CBS = "NORM.CreateCbs"
	NORM_ATTACH_CBS = "NORM.AttachCbs"
	NORM_DETACH_CBS = "NORM.DetachCbs"
	NORM_RENEW_CBS = "NORM.RenewCbs"
	NORM_QUERY_CBS = "NORM.QueryCbs"

	NORM_CBS_INTERFACE_CREATE_DISK = "qcloud.cbs.TradeWrapperForYunApi"
	NORM_CBS_INTERFACE_ATTACH_DISK = "qcloud.cbs.AttachCbsInstance"
	NORM_CBS_INTERFACE_DETACH_DISK = "qcloud.cbs.DetachCbsInstance"
	NORM_CBS_INTERFACE_QUERY_TASK = "qcloud.cbs.queryTask"
	NORM_CBS_INTERFACE_LIST_DISK = "qcloud.cbs.ListCbsInstance"
	NORM_CBS_INTERFACE_RENEW_DISK = "qcloud.cbs.SetCbsAutoRenewFlag"

	ENV_CLUSTER_ID = "CLUSTER_ID"
	ENV_APP_ID = "APPID"
)

type NormRequest map[string]interface{}

type NormResponse struct {
	Code     int         `json:"returnValue"`
	Msg      string      `json:"returnMsg"`
	Version  string      `json:"version"`
	Password string      `json:"password"`
	Data     interface{} `json:"returnData"`
}

type NormNoRsp struct {
}

type NormClient struct {
	httpClient
	request  NormRequest
	response NormResponse
}

type normDetail struct {
	Detail interface{} `json:"detail"`
}

func getNormUrl() string {
	url := os.Getenv("QCLOUD_NORM_URL")
	if url == "" {
		url = "http://169.254.0.40:80/norm/api"
	}
	return url
}

func NewNormClient() *NormClient {
	c := &NormClient{
		request:  NormRequest{},
		response: NormResponse{},
	}
	c.httpClient = httpClient{
		url:     getNormUrl(),
		timeout: 10,
		caller:  "cloudprovider",
		callee:  "NORM",
		packer:  c,
	}
	return c
}

//从环境变量中获取clusterID和appId放入norm的para中 for meta cluster
func SetNormReqExt(body []byte) ([]byte) {
	js, err := simplejson.NewJson(body)
	if err != nil {
		glog.Error("SetNormReqExt NewJson error,", err)
		return nil
	}
	if os.Getenv(ENV_CLUSTER_ID) != "" {
		js.SetPath([]string{"interface", "para", "unClusterId"}, os.Getenv(ENV_CLUSTER_ID))
		glog.V(4).Info("SetNormReqExt set unClusterId", os.Getenv(ENV_CLUSTER_ID))

	}
	if os.Getenv(ENV_APP_ID) != "" {
		js.SetPath([]string{"interface", "para", "appId"}, os.Getenv(ENV_APP_ID))
		glog.V(4).Info("SetNormReqExt set appId", os.Getenv(ENV_APP_ID))

	}

	out, err := js.Encode()
	if err != nil {
		glog.Error("SetNormReqExt Encode error,", err)
		return body
	}

	return out
}

func (c *NormClient) packRequest(interfaceName string, reqObj interface{}) ([]byte, error) {
	c.request = map[string]interface{}{
		"eventId":   rand.Uint32(),
		"timestamp": time.Now().Unix(),
		"caller":    c.httpClient.caller,
		"callee":    c.httpClient.callee,
		"version":   "1",
		"password":  "cloudprovider",
		"interface": map[string]interface{}{
			"interfaceName": interfaceName,
			"para":          reqObj,
		},
	}

	b, err := json.Marshal(c.request)
	if err != nil {
		glog.Error("packRequest failed:", err, ", req:", c.request)
		return nil, err
	}
	b = SetNormReqExt(b)
	return b, nil
}

func (c *NormClient) unPackResponse(data []byte, responseData interface{}) (err error) {
	c.response.Data = responseData
	err = json.Unmarshal(data, &c.response)
	if err != nil {
		glog.Error("Unmarshal goods response err:", err)
		return

	}
	return
}

func (c *NormClient) getResult() (int, string) {
	return c.response.Code, c.response.Msg
}

type NormAddNodeReq struct {
	AppID       string  `json:"appId"`
	BMaster     int     `json:"bMaster"`
	VmPara      string  `json:"vmPara"`
	UInstanceID *string `json:"uInstanceId"`
	UnClusterID string  `json:"unClusterId"`
	UUID        string  `json:"uuid"`
	Version     string  `json:"version"`
	LanIp       string  `json:"vmip"` //TODO 这个参数干啥用的?
	VpcID       int     `json:"vpcId"`
}

func NormAddNode(req NormAddNodeReq) error {
	c := NewNormClient()
	rsp := make(map[string]string, 0)
	err := c.DoRequest(NORM_ADD_NODE, normDetail{Detail: req}, &rsp)
	if err != nil {
		glog.Error("NormAddNode err:", err.Error())
		return err
	}
	return nil
}

type NormDelNodeReq struct {
	AppID       string `json:"appId"`
	UUID        string `json:"uuid"`
	UnClusterID string `json:"unClusterId"`
}

func NormDelNode(req NormDelNodeReq) error {
	c := NewNormClient()
	rsp := make(map[string]string, 0)
	err := c.DoRequest(NORM_DEL_NODE, normDetail{Detail: req}, &rsp)
	if err != nil {
		glog.Error("NormDelNode err:%s", err.Error())
		return err
	}
	return nil
}

type NormGetNodeInfoReq struct {
}

type NormGetNodeInfoRsp struct {
	UInstanceId string `json:"uInstanceId"`
	InternalIp  string `json:"InternalIp"`
	Name        string `json:"name"`
	Uuid        string `json:"uuid"`
	RegionId    int    `json:"regionId"`
	ZoneId      int    `json:"zoneId"`
}

func NormGetNodeInfo(req NormGetNodeInfoReq) (*NormGetNodeInfoRsp, error) {
	c := NewNormClient()
	rsp := &NormGetNodeInfoRsp{}
	err := c.DoRequest(NORM_GET_NODE_INFO, req, rsp)
	if err != nil {
		glog.Errorf("norm get node info err: %s\n", err)
		return rsp, err
	}
	glog.Infof("node info: %#v\n", rsp)
	return rsp, nil
}

func NormGetNodeInfoByName(name string) (*NormGetNodeInfoRsp, error) {
	c := NewNormClient()
	rsp := &NormGetNodeInfoRsp{}
	req := make(map[string]string)
	req["name"] = name
	err := c.DoRequest(NORM_GET_NODE_INFO_BY_NAME, req, rsp)
	if err != nil {
		glog.Errorf("norm get node info(%s) err: %s\n", name, err)
		return rsp, err
	}
	glog.Infof("node info: %#v\n", rsp)
	return rsp, nil
}

type NormListRoutesReq struct {
	ClusterName string `json:"clusterName"`
}

type NormListRoutesRsp struct {
	Routes []NormRouteInfo `json:"detail"`
}

func NormListRoutes(req NormListRoutesReq) (*NormListRoutesRsp, error) {
	c := NewNormClient()
	rsp := &NormListRoutesRsp{}
	err := c.DoRequest(NORM_LIST_ROUTES, req, rsp)
	if err != nil {
		glog.Errorf("norm list route err: %s\n", err)
		return rsp, err
	}
	glog.Infof("node info: %#v\n", rsp)
	return rsp, nil
}

type NormRouteInfo struct {
	Name   string `json:"name"`
	Subnet string `json:"subnet"`
}

type NormAddRouteRsp struct {
}

func NormAddRoute(req []NormRouteInfo) (*NormAddRouteRsp, error) {
	c := NewNormClient()
	rsp := &NormAddRouteRsp{}
	err := c.DoRequest(NORM_ADD_ROUTE, normDetail{req}, rsp)
	if err != nil {
		glog.Errorf("norm add route err: %s\n", err)
		return rsp, err
	}
	glog.Infof("node info: %#v\n", rsp)
	return rsp, nil
}

type NormDelRouteRsp struct {
}

func NormDelRoute(req []NormRouteInfo) (*NormDelRouteRsp, error) {
	c := NewNormClient()
	rsp := &NormDelRouteRsp{}
	err := c.DoRequest(NORM_DEL_ROUTE, normDetail{req}, rsp)
	if err != nil {
		glog.Errorf("norm del route err: %s\n", err)
		return rsp, err
	}
	return rsp, nil
}

type NormCreateLbReq struct {
	Namespace      string `json:"namespace"`
	ServiceName    string `json:"serviceName"`
	ServiceUuid    string `json:"serviceUuid"`
	LBName         string `json:"LBName"`
	LBType         string `json:"LBType"`
	SubnetID       *int   `json:"subnetId,omitempty"`
	UniqSubnetID   *string`json:"uniqSubnetId,omitempty"`
	Vip            *string`json:"theVip,omitempty"`

	LBDomainPrefix string `json:"LBPrefix"`
}

type NormCreateLbRsp struct {
	Namespace string `json:"namespace"`
}

type CreateLbPara struct {
	LbName   string
	GoodsNum int
	VpcId    int
	SubnetId int
}

type CreateLbRequest struct {
	Platform int       `json:"platform"`
	Type     int       `json:"type"`
	Goods    []LbGoods `json:"goods"`
}

type LbGoods struct {
	AppID           int           `json:"appId"`
	GoodsCategoryID int           `json:"goodsCategoryId"`
	GoodsDetail     LbGoodsDetail `json:"goodsDetail"`
	GoodsNum        int           `json:"goodsNum"`
	ProjectID       int           `json:"projectId"`
	RegionID        uint          `json:"regionId"`
}

type LbGoodsDetail struct {
	LBInfo   []LbInfoDetail `json:"LBInfo"`
	LBType   string         `json:"LBType"`
	SubnetID int            `json:"subnetId"`
	VpcID    int            `json:"vpcId"`
}

type LbInfoDetail struct {
	LBName       string `json:"LBName"`
	DomainPrefix string `json:"domainPrefix"`
}

func NormCreateLB(namespace string, serviceName string, serviceUuid string,
lBName string, lBType string, lbDomainPrefix string, subnetId *int, uniqSubnetId *string, vip string) error {
	c := NewNormClient()
	rsp := &NormNoRsp{}
	req := NormCreateLbReq{
		Namespace:   namespace,
		ServiceName: serviceName,
		ServiceUuid: serviceUuid,
		LBName:      lBName,
		LBType:      lBType,
		LBDomainPrefix: lbDomainPrefix,
	}
	if uniqSubnetId != nil {
		req.UniqSubnetID = uniqSubnetId
	} else {
		if subnetId != nil {
			req.SubnetID = subnetId
		}
	}
	if vip != "" {
		req.Vip = &vip
	}
	err := c.DoRequest(NORM_CREATE_LB, req, rsp)
	if err != nil {
		glog.Errorf("norm create LB err: %s\n", err)
		return err
	}
	return nil
}

type NormDeleteLbReq struct {
	ServiceUuid string `json:"serviceUuid"`
}

func NormDeleteLB(serviceUuid string) error {
	c := NewNormClient()
	rsp := &NormNoRsp{}
	err := c.DoRequest(NORM_DELETE_LB, NormDeleteLbReq{ServiceUuid: serviceUuid}, rsp)
	if err != nil {
		glog.Errorf("norm create LB err: %s\n", err)
		return err
	}
	return nil
}

type NormGetLBReq struct {
	ServiceUuid string `json:"serviceUuid"`
}

type NormGetLBRsp struct {
	DealStatus  string `json:"dealStatus"`
	UnClusterId string `json:"unClusterId"`
	ULBId       string `json:"uLBId"`
	DealId      int    `json:"dealId"`
	Namespace   string `json:"namespace"`
	ServiceName string `json:"serviceName"`
	ServiceUuid string `json:"serviceUuid"`
}

const (
	LB_STATUS_NOT_EXISTS = "notExists" //还没有下过单
	LB_STATUS_CREATING = "creating"  //下过单正在创建
	LB_STATUS_NORMAL = "normal"    //创建成功了
	LB_STATUS_CREATE_FAIL = "abnormal"  //下单后创建失败了
)

func NormGetLBCreateStatus(serviceUuid string) (*NormGetLBRsp, error) {
	c := NewNormClient()
	rsp := &NormGetLBRsp{}
	err := c.DoRequest(NORM_GET_LB, NormGetLBReq{ServiceUuid: serviceUuid}, rsp)
	if err != nil {
		glog.Errorf("NormGetLBCreateStatus err: %s\n", err)
		return nil, err
	}
	return rsp, nil
}

type NormCallPassReq struct {
	PassData PassRequest `json:"passData"`
}

type NormCallPassRsp struct {
	PassData map[string]interface{} `json:"passData"`
}

func NormCallPass(module string, interfaceName string, reqObj interface{}) (*NormCallPassRsp, error) {
	c := NewNormClient()
	rsp := &NormCallPassRsp{}
	err := c.DoRequest(module, NormCallPassReq{PassData: packPassRequest(interfaceName, reqObj)}, rsp)
	if err != nil {
		glog.Errorf("norm NormCallPass err: %s\n", err)
		return rsp, err
	}
	return rsp, nil
}

type PassHeader struct {
	Version       int    `json:"version"`
	ComponentName string `json:"componentName"`
	EventId       int64  `json:"eventId"`
	Timestamp     int64  `json:"Timestamp"`
	User          string `json:"user"`
}

type PassRequest struct {
	PassHeader
	Interface map[string]interface{} `json:"interface"`
}

type PassResponse struct {
	Code int         `json:"returnCode"`
	Msg  string      `json:"returnMessage"`
	Data interface{} `json:"data"`
}

type LbRspTskId struct {
	TaskID float64 `json:"taskId"`
}

func packPassRequest(interfaceName string, reqObj interface{}) PassRequest {
	var request PassRequest
	request.EventId = rand.Int63n(999999)
	request.Version = 1
	request.ComponentName = "cloudprovider" //TODO
	request.Timestamp = time.Now().Unix()
	request.Interface = make(map[string]interface{})
	request.Interface["interfaceName"] = interfaceName
	request.Interface["para"] = reqObj
	return request
}

//返回透传接口中返回json的taskid
func getTaskID(rsp NormCallPassRsp) (taskid int64, e error) {
	b, _ := json.Marshal(rsp.PassData)
	var r PassResponse
	err := json.Unmarshal(b, &r)
	if err != nil {
		glog.Errorf("can't get TaskID, data parse failed: %s", err.Error())
		return -1, err
	}
	b, err = json.Marshal(r.Data)
	if err != nil {
		glog.Errorf("cant get TaskID, data marshal failed: %s", err.Error())
		return -1, err
	}

	var ret LbRspTskId
	err = json.Unmarshal(b, &ret)
	if err != nil {
		glog.Errorf("cant get TaskID, data unmarshal to TaskId failed: %s", err.Error())
		return -1, err
	}
	return int64(ret.TaskID), nil

}

//返回透传接口中返回json的status
func getStatus(rsp NormCallPassRsp) (status int, e error) {
	b, _ := json.Marshal(rsp.PassData)
	js, err := simplejson.NewJson(b)
	if err != nil {
		return -2, err
	}
	intb, ok := js.Get("data").CheckGet("status")
	if !ok {
		return -2, errors.New("status not exists")
	}

	return intb.MustInt(-1), nil
}






//返回接口json中的字段
// returnCode, returnMessage , error
func getCodeMsg(rsp NormCallPassRsp) (int, string, error) {
	b, _ := json.Marshal(rsp.PassData)

	js, err := simplejson.NewJson(b)
	if err != nil {
		return 0, "", errors.New("simplejson.NewJson(b) filed!," + err.Error())
	}

	code, ok := js.CheckGet("returnCode")
	if !ok {
		return 0, "", errors.New("returnCode not exisit!" + err.Error())
	}
	msg, ok := js.CheckGet("returnMessage")

	if !ok {
		return 0, "", errors.New("returnCode not exisit!" + err.Error())
	}
	return code.MustInt(-1), msg.MustString("MustString failed!"), nil
}

//TODO norm返回成功,subLb失败?
//TODO norm返回成功的话,subLb成功?
//输入LBrsp类型
func unPackPassResponse(rsp NormCallPassRsp, reqObj interface{}) error {

	b, _ := json.Marshal(rsp.PassData)

	r := PassResponse{Data: reqObj}
	//TODO 获取code失败,code不为0,Unmarshal失败?
	err := json.Unmarshal(b, &r)
	return err
}

type QueryLbResponse struct {
	LBList []*LBInfo `json:"LBList"`
}

type LBInfo struct {
	LBId        int    `json:"LBId"`
	ULBId       string `json:"uLBId"`
	LBType      string `json:"LBType"`
	//AddTimestamp string   `json:"addTimestamp"`
	//ProjectID    int      `json:"projectId"`
	Status      int `json:"status"`
	VpcID       int `json:"vpcId"`
	SubnetID    int `json:"subnetId"`
	VipInfoList []struct {
		//IspID string `json:"ispId"`
		Vip string `json:"vip"`
	} `json:"vipInfoList"`
}

type QueryLbRequest struct {
	ULBIds []string `json:"uLBIds,omitempty"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

func QueryLbCommonPass(lbId string) (*LBInfo, error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.queryLBInfo", QueryLbRequest{
		ULBIds: []string{lbId},
		Offset: 0,
		Limit:  1,
	})
	if err != nil {
		return nil, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(fmt.Sprintf("query LB get code!=0,code:%d,msg:%s", code, msg))
	}

	LBRsp := &QueryLbResponse{}
	if err = unPackPassResponse(*NormRsp, LBRsp); err != nil {
		return nil, errors.New(fmt.Sprintf("unPackLBSubRequest error!%s", err.Error()))
	}
	if len(LBRsp.LBList) != 1 {
		return nil, errors.New(fmt.Sprintf("len(LBRsp.LBList) is %d,not equel 1,errors", len(LBRsp.LBList)))

	}
	return LBRsp.LBList[0], nil

}
func GetLbInfo(serviceUuid string) (queryRsp *LBInfo, exists bool, err error) {
	rsp, err := NormGetLBCreateStatus(serviceUuid)
	if err != nil {
		glog.Errorf("GetLbInfo(%s) error!!err:%s", serviceUuid, err.Error())

		return nil, false, err
	}
	if rsp.DealStatus == LB_STATUS_NOT_EXISTS {
		glog.Infof("status is not Exists!")

		return nil, false, nil
	}
	if rsp.DealStatus != LB_STATUS_NORMAL {
		glog.Infof("(%s) is not create succ!status:%s", serviceUuid, rsp.DealStatus)
		//存在但是还没创建成功
		return nil, true, nil
	}

	queryRsp, err = QueryLbCommonPass(rsp.ULBId)

	if err != nil {
		glog.Infof("QueryLbCommonPass(%s) error!!err:%s", rsp.ULBId, err.Error())
		return nil, true, err
	}
	return queryRsp, true, nil
}

type QueryListenInfoReq struct {
	ULBId string `json:"loadBalanceId"`
}

type ListerInfo struct {
	PortMapId     int         `json:"portMapId"`
	UListenerId   string      `json:"uListenerId"`
	ListenerName  string      `json:"listenerName"`
	LBId          int         `json:"LBId"`
	Protocol      string      `json:"protocol"`
	Vport         int32       `json:"vport"` //LB port
	Pport         int32       `json:"pport"`
	Status        int         `json:"status"`
	SessionExpire int         `json:"sessionExpire"`
	CertId        string      `json:"certId"`
	CertCaId      string      `json:"certCaId"`
	SSLMode       interface{} `json:"SSLMode"`
	HealthSwitch  bool        `json:"healthSwitch"`
	TimeOut       int         `json:"timeOut"`
	IntervalTime  int         `json:"intervalTime"`
	HealthNum     int         `json:"healthNum"`
	UnhealthNum   int         `json:"unhealthNum"`
	HttpHash      string      `json:"httpHash"`
	HttpGzip      bool        `json:"httpGzip"`
	HttpCheckPath string      `json:"httpCheckPath"`
	HttpCode      int         `json:"httpCode"`
	HealthStatus  int         `json:"healthStatus"`
	AddTimestamp  string      `json:"addTimestamp"`
	ModTimestamp  string      `json:"modTimestamp"`
}

func GetLbListerInfo(ULBId string) (LBRsp []*ListerInfo, err error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.queryLBPortInfo", QueryListenInfoReq{
		ULBId: ULBId,
	})
	if err != nil {
		return nil, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(fmt.Sprintf("query LB get code!=0,code:%d,msg:%s", code, msg))
	}

	LBRsp = make([]*ListerInfo, 0)
	if err = unPackPassResponse(*NormRsp, &LBRsp); err != nil {
		return nil, errors.New(fmt.Sprintf("unPackLBSubRequest error!%s", err.Error()))
	}
	return LBRsp, nil

}

func convertLbProtocol(protocol string) (string) {
	switch protocol {
	case "TCP", "tcp":
		return LB_LISTENER_PROTOCAL_TCP
	case "UDP", "udp":
		return LB_LISTENER_PROTOCAL_UDP
	default:
		return LB_LISTENER_PROTOCAL_TCP
	}
}

const (
	LB_LISTENER_PROTOCAL_TCP = "tcp"
	LB_LISTENER_PROTOCAL_UDP = "udp"
)

type CreateListenerRequest struct {
	LoadBalanceId string        `json:"loadBalanceId"`
	New7          int           `json:"new7"`
	ProjectId     int           `json:"projectId"`
	PortMap       []PortMapInfo `json:"portMap"`
}

type PortMapInfo struct {
	Vport        int32  `json:"vport"`    //LB port
	Pport        int32  `json:"pport"`    // target port
	Protocol     string `json:"protocol"` //tcp or udp
	ListenerName string `json:"listenerName"`
}

type CommonLbResponse struct {
	TaskID int `json:"taskId"`
}

func CreateLbLister(ULBId string, PortMap []PortMapInfo) (int64, error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.addLBPortMap", CreateListenerRequest{
		LoadBalanceId: ULBId,
		ProjectId:     0,
		New7:          1,
		PortMap:       PortMap,
	})
	if err != nil {
		return -1, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return -1, err
	}
	if code != 0 {
		return -1, errors.New(fmt.Sprintf("create LB listener get code!=0,code:%d,msg:%s", code, msg))
	}

	var e error
	taskid, err := getTaskID(*NormRsp)
	if e != nil {
		glog.Errorf("CreateLbLister can't get taskid")
	}

	return taskid, nil
}

type DeleteListenerRequest struct {
	LoadBalanceId string   `json:"loadBalanceId"`
	UlistenerIds  []string `json:"ulistenerIds"`
}

func DeleteLbLister(ULBId string, listenerIds []string) (int64, error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.deleteListeners", DeleteListenerRequest{
		LoadBalanceId: ULBId,
		UlistenerIds:  listenerIds,
	})
	if err != nil {
		return -1, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return -1, err
	}
	if code != 0 {
		return -1, errors.New(fmt.Sprintf("delete LB listener get code!=0,code:%d,msg:%s", code, msg))
	}

	var e error
	taskid, err := getTaskID(*NormRsp)
	if e != nil {
		glog.Errorf("DeleteLB can't get taskid")
	}

	return taskid, nil
}

type QueryTaskResultRequest struct {
	TaskId int64    `json:"taskId"`
}

func QueryTaskResult(taskid int64) (int, error) {
	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.queryTaskResult", QueryTaskResultRequest{
		TaskId: taskid,
	})
	if err != nil {
		return -1, err
	}
	status, err := getStatus(*NormRsp)
	if err != nil {
		return -1, err
	}
	return status, nil
}

//TODO DeleteLbListerByPort

//func DeleteLbListerByPort(ULBId string, listenerIds []string) (err error) {
//
//	lbRsp, err := GetLbListerInfo(ULBId)
//
//	NormRsp, err := NormLbCallPass("qcloud.QLBNew.deleteListeners", DeleteListenerRequest{
//		LoadBalanceId:  ULBId,
//		UlistenerIds :listenerIds,
//	})
//	if err != nil {
//		return err
//	}
//	code, msg, err := getCodeMsg(*NormRsp)
//	if err != nil {
//		return err
//	}
//	if code != 0 {
//		return errors.New(fmt.Sprintf("delete LB listener get code!=0,code:%d,msg:%s", code, msg))
//	}
//	return nil
//}

func HasElem(s interface{}, elem interface{}) bool {
	arrV := reflect.ValueOf(s)

	if arrV.Kind() == reflect.Slice {
		for i := 0; i < arrV.Len(); i++ {
			if arrV.Index(i).Interface() == elem {
				return true
			}
		}
	}
	return false
}

type BindCvmRequest struct {
	Device []DeviceInfo `json:"device"`
}

type DeviceInfo struct {
	LoadBalanceId string         `json:"loadBalanceId"`
	UuidList      []string       `json:"device"`
	Weight        map[string]int `json:"weight,omitempty"`
}

type UnBindCvmRequest struct {
	Device []UnDeviceInfo `json:"device"`
}

type UnDeviceInfo struct {
	LoadBalanceId string   `json:"loadBalanceId"`
	UuidList      []string `json:"device"`
}

func BindCvmToLb(ULBId string, uuids []string) (int64, error) {
	request := &BindCvmRequest{
		Device: []DeviceInfo{
			{
				LoadBalanceId: ULBId,
				UuidList:      uuids,
			},
		},
	}
	request.Device[0].Weight = make(map[string]int, len(uuids))
	//默认全部给10
	for _, ins := range uuids {
		request.Device[0].Weight[ins] = 10
	}

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.addHost", request)
	if err != nil {
		return -1, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return -1, err
	}
	if code != 0 {
		return -1, errors.New(fmt.Sprintf("BindCvmToLb  get code!=0,code:%d,msg:%s", code, msg))
	}

	var e error
	taskid, err := getTaskID(*NormRsp)
	if e != nil {
		glog.Errorf("DeleteLB can't get taskid")
	}

	return taskid, nil
}

func UnBindCvmToLb(ULBId string, uuids []string) (int64, error) {
	request := &UnBindCvmRequest{
		Device: []UnDeviceInfo{
			{
				LoadBalanceId: ULBId,
				UuidList:      uuids,
			},
		},
	}

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.deleteHost", request)
	if err != nil {
		return -1, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return -1, err
	}
	if code != 0 {
		return -1, errors.New(fmt.Sprintf("UnBindCvmToLb  get code!=0,code:%d,msg:%s", code, msg))
	}

	var e error
	taskid, err := getTaskID(*NormRsp)
	if e != nil {
		glog.Errorf("DeleteLB can't get taskid")
	}

	return taskid, nil
}

type DeleteLbRequest struct {
	LoadBalanceIds []string `json:"loadBalanceIds"`
}

func DeleteLB(ULBId string) (int64, error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.deleteLB", DeleteLbRequest{
		LoadBalanceIds: []string{ULBId},
	})
	if err != nil {
		return -1, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return -1, err
	}
	if code != 0 {
		return -1, errors.New(fmt.Sprintf("DeleteLB  get code!=0,code:%d,msg:%s", code, msg))
	}

	var e error
	taskid, err := getTaskID(*NormRsp)
	if e != nil {
		glog.Errorf("DeleteLB can't get taskid")
	}

	return taskid, nil
}

type QueryLbHostResponse struct {
	DeviceLanIP string `json:"deviceLanIp"`
	Runflag     int    `json:"runflag"`
	Status      int    `json:"status"`
	SubnetID    int    `json:"subnetId"`
	UUID        string `json:"uuid"`
	UInstanceID string `json:"uInstanceId"`
}

type QueryLbHostRequest struct {
	ULBId string `json:"loadBalanceId"`
}

func GetLBHost(ULBId string) (uuids []string, err error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_LB, "qcloud.QLBNew.queryLBHost", QueryLbHostRequest{
		ULBId: ULBId,
	})
	if err != nil {
		return nil, err
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(fmt.Sprintf("query LB get code!=0,code:%d,msg:%s", code, msg))
	}

	LBRsp := make([]QueryLbHostResponse, 0)
	if err = unPackPassResponse(*NormRsp, &LBRsp); err != nil {
		return nil, errors.New(fmt.Sprintf("unPackLBSubRequest error!%s", err.Error()))
	}
	for _, v := range LBRsp {
		uuids = append(uuids, v.UUID)
	}
	return uuids, nil

}

func InstancesDiff(instancesFromLb []string, instancesFromK8s []string) (notInK8s []string, notInLb []string) {

	for _, insLb := range instancesFromLb {

		if !HasElem(instancesFromK8s, insLb) {
			notInK8s = append(notInK8s, insLb)
		}
	}

	for _, insK8s := range instancesFromK8s {
		if !HasElem(instancesFromLb, insK8s) {
			notInLb = append(notInLb, insK8s)
		}
	}

	return notInK8s, notInLb
}

type CreateCbsRequest struct {
	ZoneId      int    `json:"zoneId"`
	ReginId     int    `json:"regionId"`
	InquiryType string `json:"inquiryType"`
	PayMode     string `json:"payMode"`
	NewFrame    int    `json:"newFrame"`
	DiskSize    int    `json:"diskSize"`
	Count       int    `json:"count"`
	TimeSpan    int    `json:"timeSpan"`
	MediumType  string `json:"mediumType"`
}

type CbsDisk struct {
	DiskId             string
	InstanceIdAttached string
	LifeState          string
}

type CbsResources struct {
	Resources map[string][]string `json:"resourceIds"`
}

type AttachCbsDiskRequest struct {
	VmInstanceId string   `json:"uInstanceId"`
	DiskList     []string `json:"cbsInstanceIdList"`
}

type DetachCbsDiskRequest struct {
	DiskList []string `json:"cbsInstanceIdList"`
}

type AttachDetachResult struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type AttachDetachDiskResponse struct {
	TaskId int `json:"taskId"`
}

type GetCbsDiskRequest struct {
	DiskIds     []string `json:"cbsInstanceIdList"`
	InstanceIds []string `json:"uInstanceIdList"`
	LifeState   []string `json:"lifeState"`
	ProjectId   int      `json:"projectId"`
}

type CbsItem struct {
	Type       string `json:"volumeType"`
	DiskSize   int    `json:"diskSize"`
	DiskId     string `json:"cbsInstanceId"`
	LifeState  string `json:"lifeState"`
	InstanceId string `json:"uInstanceId"`
}

type GetCbsDiskResponse struct {
	Items []CbsItem `json:"items"`
}

type SetCbsAutoRenewRequest struct {
	AutoRenew int      `json:"autoRenewFlag"`
	Uuids     []string `json:"uuids"`
	Type      string   `json:"type"`
}

type SetCbsAutoRenewResponse struct {
}

type QueryCbsTaskRequest struct {
	TaskId int `json:"taskId"`
}

type QueryCbsTaskResponse struct {
	Status string `json:"status"`
}

func CreateCbsDisk(capacity int, mediumType string, region int, zone int) (diskId string, err error) {
	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_CBS, NORM_CBS_INTERFACE_CREATE_DISK, CreateCbsRequest{
		InquiryType: "create",
		PayMode:     "prePay",
		DiskSize:    capacity,
		Count:       1,
		TimeSpan:    1,
		MediumType:  mediumType,
		ReginId:     region,
		ZoneId:      zone,
	})
	if err != nil {
		return
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return
	}
	if code != 0 {
		return "", errors.New(fmt.Sprintf("createCbsDisk get code!=0,code:%d,msg:%s", code, msg))
	}

	cbsResources := &CbsResources{}
	if err = unPackPassResponse(*NormRsp, &cbsResources); err != nil {
		return "", errors.New(fmt.Sprintf("unPackCbsRequest error!%s", err.Error()))
	}

	if len(cbsResources.Resources) != 1 {
		return "", errors.New("CreateCbsDisk failed, no disk id returned.")
	}

	for _, v := range cbsResources.Resources {
		if len(v) > 0 {
			diskId = v[0]
		}
	}

	return
}

type DiskAttachDetachResult struct {
	Code    int `json:"code"`
	Message string `json:"message"`
}

func parseAttachDetachResult(normRsp *NormCallPassRsp, diskId string) (int, int, string, error) {
	if normRsp == nil {
		return 0, 0, "", fmt.Errorf("parseAttachDetachResult error, rsp is nil, diskId:%s", diskId)
	}

	m, _ := normRsp.PassData["data"]
	diskResultMap := m.(map[string]interface{})
	if diskResultMap == nil {
		return 0, 0, "", errors.New("AttachDetach result data is invalid")
	}

	t, ok := diskResultMap["taskId"]
	if !ok {
		return 0, 0, "", errors.New("result hasn't taskId")
	}

	var taskId int
	switch i := t.(type) {
	case float32:
		taskId = int(i)
	case float64:
		taskId = int(i)
	case int32:
		taskId = int(i)
	case int64:
		taskId = int(i)
	default:
		return 0, 0, "", errors.New("taskId is invalid")
	}

	d, ok := diskResultMap[diskId]
	if !ok {
		return 0, 0, "", errors.New("no disk found in result")
	}

	var result DiskAttachDetachResult
	data, _ := json.Marshal(d)
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, 0, "", errors.New("disk result is invalid")
	}

	return taskId, result.Code, result.Message, nil
}

func AttachCbsDisk(diskId string, vmInstanceId string) (string, error) {
	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_CBS, NORM_CBS_INTERFACE_ATTACH_DISK, AttachCbsDiskRequest{
		VmInstanceId: vmInstanceId,
		DiskList:     []string{diskId},
	})
	if err != nil {
		return "", fmt.Errorf("AttachCbsDisk failed, error:%v, instance:%s, diskId:%s", err, vmInstanceId, diskId)
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return "", fmt.Errorf("AttachCbsDisk failed, error:%v, instance:%s, diskId:%s", err, vmInstanceId, diskId)
	}
	if code != 0 {
		return "", fmt.Errorf("AttachCbsDisk failed, code:%d, message:%s, instance:%s, diskId:%s",
			code, msg, vmInstanceId, diskId)
	}

	taskId, code, message, err := parseAttachDetachResult(NormRsp, diskId)
	if err != nil {
		return "", fmt.Errorf("parseAttachDetachResult failed:%v, diskId:%s, vmInstance:%s",
			err, diskId, vmInstanceId)
	}

	if code != 0 {
		return "", fmt.Errorf("AttachCbsDisk failed, code:%d, message:%s, instance:%s, diskId:%s",
			code, message, vmInstanceId, diskId)
	}

	err = wait.Poll(3 * time.Second, 60 * time.Second, func() (bool, error) {
		status, err := QueryCbsTask(taskId)
		if err != nil {
			return false, err
		}

		if status == "success" {
			return true, nil
		} else if status == "failed" {
			return false, errors.New("QueryCbsTask, attach disk status failed")
		} else if status == "running" {
			return false, nil
		}

		return false, fmt.Errorf("QueryCbsTask, attach disk failed, status:%s, error:%v", status, err)
	})

	if err != nil {
		return "", fmt.Errorf("AttachCbsDisk poll task failed. disk:%s, node:%s, error:%v",
			diskId, vmInstanceId, err)
	}

	return fmt.Sprintf(" /dev/disk/by-id/virtio-%s", diskId), nil
}

func DetachCbsDisk(diskId string) error {
	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_CBS, NORM_CBS_INTERFACE_DETACH_DISK, DetachCbsDiskRequest{
		DiskList: []string{diskId},
	})
	if err != nil {
		return fmt.Errorf("DetachCbsDisk failed, error:%v, diskId:%s", err, diskId)
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return fmt.Errorf("DetachCbsDisk failed, error:%v, diskId:%s", err, diskId)
	}
	if code != 0 {
		return fmt.Errorf("DetachCbsDisk failed, code:%d, message:%s, diskId:%s",
			code, msg, diskId)
	}

	taskId, code, message, err := parseAttachDetachResult(NormRsp, diskId)
	if err != nil {
		return fmt.Errorf("parseAttachDetachResult failed:%v, diskId:%s", err, diskId)
	}

	if code != 0 {
		return fmt.Errorf("AttachCbsDisk failed, code:%d, message:%s, diskId:%s",
			code, message, diskId)
	}

	err = wait.Poll(3 * time.Second, 60 * time.Second, func() (bool, error) {
		status, err := QueryCbsTask(taskId)
		if err != nil {
			return false, err
		}

		if status == "success" {
			return true, nil
		} else if status == "failed" {
			return false, errors.New("QueryCbsTask, detach disk status failed")
		} else if status == "running" {
			return false, nil
		}

		return false, fmt.Errorf("QueryCbsTask detach disk failed, status:%s, error:%v", status, err)
	})

	if err != nil {
		return fmt.Errorf("DetachCbsDisk, poll task failed,  error:%v, diskId:%s", err, diskId)
	}

	return nil
}

func GetCbsDisk(diskId string) (*CbsDisk, error) {

	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_CBS, NORM_CBS_INTERFACE_LIST_DISK, GetCbsDiskRequest{
		DiskIds:   []string{diskId},
		ProjectId: 0,
		LifeState: []string{"normal"},
	})

	if err != nil {
		return nil, fmt.Errorf("GetCbsDisk failed, error:%v, diskId:%s", err, diskId)
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return nil, fmt.Errorf("GetCbsDisk failed, error:%v, diskId:%s", err, diskId)
	}
	if code != 0 {
		return nil, fmt.Errorf("GetCbsDisk failed, code:%d, message:%s, diskId:%s",
			code, msg, diskId)
	}
	var getCbsDiskRsp GetCbsDiskResponse
	if err = unPackPassResponse(*NormRsp, &getCbsDiskRsp); err != nil {
		return nil, fmt.Errorf("GetCbsDisk failed, bad response. error:%v, diskId:%s", err, diskId)
	}

	if len(getCbsDiskRsp.Items) == 0 {
		return nil, nil
	}

	item := &getCbsDiskRsp.Items[0]
	disk := &CbsDisk{
		DiskId:             item.DiskId,
		LifeState:          item.LifeState,
		InstanceIdAttached: item.InstanceId,
	}

	return disk, nil
}

func SetCbsRenewFlag(diskId string, flag int) error {
	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_CBS, NORM_CBS_INTERFACE_RENEW_DISK, SetCbsAutoRenewRequest{
		Uuids:     []string{diskId},
		Type:      "cbs",
		AutoRenew: flag,
	})
	if err != nil {
		return fmt.Errorf("SetCbsRenewFlag failed, error:%v, diskId:%s, flag:%d", err, diskId, flag)
	}
	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return fmt.Errorf("SetCbsRenewFlag failed, error:%v, diskId:%s, flag:%d", err, diskId, flag)
	}
	if code != 0 {
		return fmt.Errorf("SetCbsRenewFlag failed, code:%d, message:%s, diskId:%s, flag:%d",
			code, msg, diskId, flag)
	}
	var setCbsAutoRenewResponse SetCbsAutoRenewResponse
	if err = unPackPassResponse(*NormRsp, &setCbsAutoRenewResponse); err != nil {
		return fmt.Errorf("SetCbsRenewFlag failed, bad response. error:%v, diskId:%s, flag:%d", err, diskId, flag)
	}

	return nil
}

func QueryCbsTask(taskId int) (string, error) {
	NormRsp, err := NormCallPass(NORM_CALL_PASS_MODULE_CBS, NORM_CBS_INTERFACE_QUERY_TASK, QueryCbsTaskRequest{
		TaskId: taskId,
	})

	if err != nil {
		return "", fmt.Errorf("QueryCbsTask failed, taskId:%d, error:%v", taskId, err)
	}

	code, msg, err := getCodeMsg(*NormRsp)
	if err != nil {
		return "", fmt.Errorf("QueryCbsTask failed, code:%d, msg:%s, taskId:%d, error:%v", code, msg, taskId, err)
	}

	if code != 0 {
		return "", fmt.Errorf("QueryCbsTask return code:%d, taskId:%d, error:%v", code, taskId, err)
	}

	response := &QueryCbsTaskResponse{}
	if err = unPackPassResponse(*NormRsp, response); err != nil {
		return "", fmt.Errorf("unPackPassResponse failed, bad response. rsp:%v, error:%v", *NormRsp, err)
	}

	return response.Status, nil
}

type NormGetAgentCredentialReq struct {
	Duration int `json:"duration"`
}

type NormGetAgentCredentialRsp struct {
	Credentials struct {
					Token        string `json:"token"`
					TmpSecretId  string `json:"tmpSecretId"`
					TmpSecretKey string `json:"tmpSecretKey"`
				} `json:"credentials"`
	ExpiredTime int `json:"expiredTime"`
}

func NormGetAgentCredential(req NormGetAgentCredentialReq) (*NormGetAgentCredentialRsp, error) {
	c := NewNormClient()
	rsp := &NormGetAgentCredentialRsp{}
	err := c.DoRequest(NORM_GET_AGENT_CREDENTIAL, req, rsp)
	if err != nil {
		glog.Errorf("norm get node info err: %s\n", err)
		return rsp, err
	}
	glog.Infof("node info: %#v\n", rsp)
	return rsp, nil
}