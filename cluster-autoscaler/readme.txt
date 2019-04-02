#配置GOPATH，编译
go build -o cluster-autoscaler

#打包
docker build -t ccr.ccs.tencentyun.com/ccs-dev/cluster-autoscaler:v1.12-tke.2.1.open . --no-cache