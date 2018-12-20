package scaling

import (
	"os"

	"github.com/dbdd4us/qcloudapi-sdk-go/common"
)

const (
	ScalingHost = "scaling.api.qcloud.com"
	ScalingPath = "/v2/index.php"
)

type Client struct {
	*common.Client
}

func NewClient(credential common.CredentialInterface, opts common.Opts) (*Client, error) {
	if opts.Host == "" {
		opts.Host = ScalingHost
	}
	if opts.Path == "" {
		opts.Path = ScalingPath
	}

	client, err := common.NewClient(credential, opts)
	if err != nil {
		return &Client{}, err
	}
	return &Client{client}, nil
}

func NewClientFromEnv() (*Client, error) {

	secretId := os.Getenv("QCloudSecretId")
	secretKey := os.Getenv("QCloudSecretKey")
	region := os.Getenv("QCloudScalingAPIRegion")
	host := os.Getenv("QCloudScalingAPIHost")
	path := os.Getenv("QCloudScalingAPIPath")

	return NewClient(
		common.Credential{
			SecretId: secretId,
			SecretKey:secretKey,
		},
		common.Opts{
			Region: region,
			Host:   host,
			Path:   path,
		},
	)
}

