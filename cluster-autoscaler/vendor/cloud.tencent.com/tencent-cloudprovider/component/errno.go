package component

import (
	"fmt"
)

type Errno struct {
	Code int
	Msg  string
}

func (e Errno) GenMsg(msg string) string {
	return e.Msg + "[" + msg + "]"
}

var SUCCESS = Errno{Code: 0, Msg: "SUCCESS"}

var PARAM_ERROR = Errno{Code: -10000, Msg: "PARAM_ERROR"}
var ACTION_REQUIRE_ERROR = Errno{Code: -10001, Msg: "ACTION_REQUIRE_ERROR"}
var CREATE_MASTER_FAILED_ERROR = Errno{Code: -10002, Msg: "CREATE_MASTER_FAILED_ERROR"}

//DB相关错误
var DB_ERROR = Errno{Code: -20000, Msg: "DB_ERROR"} //gorm抛出未知错误
var DB_RECORD_NOT_FOUND = Errno{Code: -20001, Msg: "DB_RECORD_NOT_FOUND"} //单资源的query中,没有查到record
var DB_AFFECTIVED_ROWS_ERROR = Errno{Code: -20002, Msg: "DB_AFFECTIVED_ROWS_ERROR"} //update或者事物中,生效的条目数量不对


//http client相关错误
var COMPONENT_CLINET_COMMON_ERROR = Errno{Code: -30000, Msg: "COMPONENT_CLINET_COMMON_ERROR"}
var COMPONENT_CLINET_HTTP_ERROR = Errno{Code: -30001, Msg: "COMPONENT_CLINET_HTTP_ERROR"}
var COMPONENT_CLINET_TIMEOUT_ERROR = Errno{Code: -30002, Msg: "COMPONENT_CLINET_TIMEOUT_ERROR"}
var COMPONENT_CLINET_UNPACK_ERROR = Errno{Code: -30002, Msg: "COMPONENT_CLINET_UNPACK_ERROR"}

var MATRIX_COMMON_ERROR = Errno{Code: -110000, Msg: "MATRIX_COMMON_ERROR"}

var TRADE_COMMON_ERROR = Errno{Code: -120000, Msg: "TRADE_COMMON_ERROR"}

var CVM_COMMON_ERROR = Errno{Code: -130000, Msg: "CVM_COMMON_ERROR"}

var NORM_COMMON_ERROR = Errno{Code: -140000, Msg: "NORM_COMMON_ERROR"}

var VPC_COMMON_ERROR = Errno{Code: -150000, Msg: "VPC_COMMON_ERROR"}

var ETCD_COMMON_ERROR = Errno{Code: -160000, Msg: "ETCD_COMMON_ERROR"}

//TODO DashboardError中增加 嵌套的DashboardError类型,用来一层层传递错误
//如 requestResultError -> norm Error ->master create Error
type DashboardError struct {
	Code int
	Msg  string
	Err  error
}

func (e DashboardError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("DashboardError,Code : %d , Msg : %s, err : %s", e.Code, e.Msg, e.Err.Error())
	}
	return fmt.Sprintf("DashboardError,Code : %d , Msg : %s, err : nil", e.Code, e.Msg)
}

func NewDashboardError(errno Errno, extraMsg string, err error) *DashboardError {
	return &DashboardError{Code:errno.Code, Msg:errno.GenMsg(extraMsg), Err:err}
}



