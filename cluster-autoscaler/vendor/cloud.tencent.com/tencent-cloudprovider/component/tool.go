package component

import (
	"fmt"
	"k8s.io/kubernetes/pkg/api/v1"
)


/*notInK8s :to delete from LB */
/*notInLb: to add to LB*/
func DiffListerPort(apiService *v1.Service, LBListeners []*ListerInfo) (UListenerIdNotInK8s []string, PortsNotInLb []PortMapInfo) {

	add := func(port int32, protocol string) string {
		return fmt.Sprintf("%s_%s", port, convertLbProtocol(protocol))
	}

	servicePortMap := make(map[string]v1.ServicePort)

	listerInfoMap := make(map[string]*ListerInfo)

	for _, servicePort := range apiService.Spec.Ports {
		servicePortMap[add(servicePort.Port, string(servicePort.Protocol))] = servicePort
	}
	for _, LBListener := range LBListeners {
		listerInfoMap[add(LBListener.Vport, LBListener.Protocol)] = LBListener
	}

	for key, servicePort := range servicePortMap {
		listerInfo, ok := listerInfoMap[key]
		if !ok {
			//如果service有,但是LB中没有,to add
			proto := convertLbProtocol(string(servicePort.Protocol))
			PortsNotInLb = append(PortsNotInLb, PortMapInfo{
				Vport:        servicePort.Port,
				Pport:        servicePort.NodePort,
				Protocol:     proto,
				ListenerName: servicePort.Name,
			})
		} else {
			if listerInfo.Pport != servicePort.NodePort {
				//如果后端nodePort不同,需要删除,再创建
				UListenerIdNotInK8s = append(UListenerIdNotInK8s, listerInfo.UListenerId)
				proto := convertLbProtocol(string(servicePort.Protocol))
				PortsNotInLb = append(PortsNotInLb, PortMapInfo{
					Vport:        servicePort.Port,
					Pport:        servicePort.NodePort,
					Protocol:     proto,
					ListenerName: servicePort.Name,
				})

			}
		}

	}

	for key, listerInfo := range listerInfoMap {
		_, ok := servicePortMap[key]
		if !ok {
			//如果在lb中有,但是在service中没有,to del
			UListenerIdNotInK8s = append(UListenerIdNotInK8s, listerInfo.UListenerId)
		}
		//此处不需要再判断如果有,nodeport不同怎么办,因为上边已经判断过了
	}
	return UListenerIdNotInK8s, PortsNotInLb

}