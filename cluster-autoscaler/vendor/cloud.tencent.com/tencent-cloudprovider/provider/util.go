package qcloud

import (
	"fmt"
	"github.com/dbdd4us/qcloudapi-sdk-go/clb"
	"k8s.io/kubernetes/pkg/api/v1"
	"reflect"
	"time"
)

func RetryDo(op func() (interface{}, error), checker func(interface{}, error) bool, timeout uint64, interval uint64) (ret interface{}, err error, isTimeout bool) {

	isTimeout = false
	var tm <-chan time.Time
	tm = time.After(time.Duration(timeout) * time.Second)

	times := 0
	for {
		times = times + 1
		select {
		case <-tm:
			isTimeout = true
			return
		default:
		}
		ret, err = op()
		if checker(ret, err) {
			//如果成功了
			return
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return
}

func convertToLbProtocol(protocol string) int {
	switch protocol {
	case "TCP", "tcp":
		return clb.LoadBalanceListenerProtocolTCP
	case "UDP", "udp":
		return clb.LoadBalanceListenerProtocolUDP
	default:
		return clb.LoadBalanceListenerProtocolTCP
	}
}

/*notInK8s :to delete from LB */
/*notInLb: to add to LB*/
func DiffListerPort(apiService *v1.Service, LBListeners *[]clb.Listener) (UListenerIdNotInK8s []string, PortsNotInLb []clb.CreateListenerOpts) {

	add := func(port int32, protocol int) string {
		return fmt.Sprintf("%d_%d", port, protocol)
	}

	servicePortMap := make(map[string]v1.ServicePort)

	listerInfoMap := make(map[string]*clb.Listener)

	for _, servicePort := range apiService.Spec.Ports {
		servicePortMap[add(servicePort.Port, convertToLbProtocol(string(servicePort.Protocol)))] = servicePort
	}
	for _, LBListener := range *LBListeners {
		listerInfoMap[add(LBListener.LoadBalancerPort, LBListener.Protocol)] = &LBListener
	}

	for key, servicePort := range servicePortMap {
		listerInfo, ok := listerInfoMap[key]
		if !ok {
			//如果service有,但是LB中没有,to add
			proto := convertToLbProtocol(string(servicePort.Protocol))
			PortsNotInLb = append(PortsNotInLb, clb.CreateListenerOpts{
				LoadBalancerPort: servicePort.Port,
				InstancePort:     servicePort.NodePort,
				Protocol:         proto,
				ListenerName:     &servicePort.Name,
			})
		} else {
			if listerInfo.InstancePort != servicePort.NodePort {
				//如果后端nodePort不同,需要删除,再创建
				UListenerIdNotInK8s = append(UListenerIdNotInK8s, listerInfo.UnListenerId)
				proto := convertToLbProtocol(string(servicePort.Protocol))
				PortsNotInLb = append(PortsNotInLb, clb.CreateListenerOpts{
					LoadBalancerPort: servicePort.Port,
					InstancePort:     servicePort.NodePort,
					Protocol:         proto,
					ListenerName:     &servicePort.Name,
				})

			}
		}

	}

	for key, listerInfo := range listerInfoMap {
		_, ok := servicePortMap[key]
		if !ok {
			//如果在lb中有,但是在service中没有,to del
			UListenerIdNotInK8s = append(UListenerIdNotInK8s, listerInfo.UnListenerId)
		}
		//此处不需要再判断如果有,nodeport不同怎么办,因为上边已经判断过了
	}
	return UListenerIdNotInK8s, PortsNotInLb

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

func stringIn(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
