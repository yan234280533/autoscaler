package component

import (
	"bytes"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"time"
	"fmt"
)



//对方返回的请求中,code部分不为0,此时的code为对方返回的code,含义不等于内部Err中的code
type RequestResultError struct {
	Code int
	Msg  string
}

func (c RequestResultError) IsSucc() bool {
	return c.Code == 0
}

func (c RequestResultError) Error() string {
	return fmt.Sprintf("RequestInvokeError,Code : %d , Msg : %s", c.Code, c.Msg)
}

func newRequestResponseError(code int, msg string) *RequestResultError {
	return &RequestResultError{
		Code: code,
		Msg: msg,
	}
}

type packer interface {
	packRequest(interfaceName string, reqObj interface{}) ([]byte, error)
	unPackResponse(data []byte, responseObj interface{}) error
	getResult() (int, string)
}

type httpClient struct {
	url     string
	timeout uint
	caller  string
	callee  string
	packer
}

func (c *httpClient) DoRequest(interfaceName string, request interface{}, response interface{}) (responseErr error) {

	requestData, err := c.packRequest(interfaceName, request)
	if err != nil {
		responseErr = NewDashboardError(COMPONENT_CLINET_COMMON_ERROR, "DoRequest failed!packRequest failed", err)
		return
	}
	httpRequest, err := http.NewRequest("POST", c.url, bytes.NewReader(requestData))
	if err != nil {
		responseErr = NewDashboardError(COMPONENT_CLINET_COMMON_ERROR, "DoRequest failed!new http request failed", err)
		return
	}
	glog.Info("Http request to: ", c.url, ", interface: ", interfaceName, ", request: ", string(requestData))

	http.DefaultClient.Timeout = time.Duration(time.Duration(c.timeout) * time.Second)
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	defer func() {
		if httpResponse != nil {
			httpResponse.Body.Close()
		}
	}()
	if err != nil {
		responseErr = NewDashboardError(COMPONENT_CLINET_HTTP_ERROR, "DoRequest failed!,do http request failed", err)
		return
	}

	if httpResponse.StatusCode != 200 {
		responseErr = NewDashboardError(COMPONENT_CLINET_HTTP_ERROR, "Http response error, code :" + httpResponse.Status, nil)
		return
	}
	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		msg := "DoRequest failed!read http response failed:" + err.Error() + ", url: " + c.url + ", interface: " + interfaceName
		glog.Error(msg)
		responseErr = NewDashboardError(COMPONENT_CLINET_HTTP_ERROR, msg, err)

		return
	}

	glog.Info("Http response from: ", c.url, ", interface: ", interfaceName, ", response: ", string(body))

	err = c.unPackResponse(body, response)
	if err != nil {
		msg := fmt.Sprint("DoRequest failed! Unmarshal response err:", err)
		glog.Error(msg)
		responseErr = NewDashboardError(COMPONENT_CLINET_UNPACK_ERROR, msg, err)
		return
	}
	code, msg := c.getResult()
	if code != 0 {
		responseErr = newRequestResponseError(code, msg)
		glog.Error("interfaceName:", interfaceName, " c.getResult code: ", code, "msg:", msg)
		return
	}
	return nil
}
