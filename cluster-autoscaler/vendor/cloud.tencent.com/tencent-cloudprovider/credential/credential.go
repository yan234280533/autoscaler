package credential

import (
	"sync"
	"time"

	"cloud.tencent.com/tencent-cloudprovider/component"
	"github.com/dbdd4us/qcloudapi-sdk-go/common"
)

type Refresher interface {
	Refresh() (string, string, string, int, error)
}

type NormRefresher struct {
	expiredDuration time.Duration
}

func NewNormRefresher(expiredDuration time.Duration) (NormRefresher, error) {
	return NormRefresher{
		expiredDuration: expiredDuration,
	}, nil
}

func (nr NormRefresher) Refresh() (secretId string, secretKey string, token string, expiredAt int, err error) {
	rsp, err := component.NormGetAgentCredential(
		component.NormGetAgentCredentialReq{
			Duration: int(nr.expiredDuration / time.Second),
		},
	)
	if err != nil {
		return "", "", "", 0, err
	}
	secretId = rsp.Credentials.TmpSecretId
	secretKey = rsp.Credentials.TmpSecretKey
	token = rsp.Credentials.Token
	expiredAt = rsp.ExpiredTime

	return
}

var _ common.CredentialInterface = &NormCredential{}

type NormCredential struct {
	secretId        string
	secretKey       string
	token           string

	expiredAt       time.Time
	expiredDuration time.Duration // TODO: maybe confused with this? find a better way

	lock            sync.Mutex

	refresher       Refresher
}

func NewNormCredential(expiredDuration time.Duration, refresher Refresher) (NormCredential, error) {
	return NormCredential{
		expiredDuration: expiredDuration,
		refresher:       refresher,
	}, nil
}

func (normCred *NormCredential) GetSecretId() (string, error) {
	if normCred.needRefresh() {
		if err := normCred.refresh(); err != nil {
			return "", err
		}
	}
	return normCred.secretId, nil
}

func (normCred *NormCredential) GetSecretKey() (string, error) {
	if normCred.needRefresh() {
		if err := normCred.refresh(); err != nil {
			return "", err
		}
	}
	return normCred.secretKey, nil
}

func (normCred *NormCredential) Values() (common.CredentialValues, error) {
	if normCred.needRefresh() {
		if err := normCred.refresh(); err != nil {
			return common.CredentialValues{}, err
		}
	}
	return common.CredentialValues{"Token": normCred.token}, nil
}

func (normCred *NormCredential) needRefresh() bool {
	return time.Now().Add(normCred.expiredDuration / time.Second / 2 * time.Second).After(normCred.expiredAt)
}

func (normCred *NormCredential) refresh() error {
	secretId, secretKey, token, expiredAt, err := normCred.refresher.Refresh()
	if err != nil {
		return err
	}

	normCred.updateExpiredAt(time.Unix(int64(expiredAt), 0))
	normCred.updateCredential(secretId, secretKey, token)

	return nil
}

func (normCred *NormCredential) updateExpiredAt(expiredAt time.Time) {
	normCred.lock.Lock()
	defer normCred.lock.Unlock()

	normCred.expiredAt = expiredAt
}

func (normCred *NormCredential) updateCredential(secretId, secretKey, token string) {
	normCred.lock.Lock()
	defer normCred.lock.Unlock()

	normCred.secretId = secretId
	normCred.secretKey = secretKey
	normCred.token = token
}
