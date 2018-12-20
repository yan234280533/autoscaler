package credential

import (
	"testing"
	"time"
)

type mockRefresher struct {
	expiredDuration time.Duration
	refreshed       bool
}

func (mrf *mockRefresher) Refresh() (string, string, string, int, error) {
	if mrf.refreshed {
		return "secret_id_refreshed", "secret_key_refreshed", "token_refreshed", int(time.Now().Add(mrf.expiredDuration).Unix()), nil
	}

	mrf.refreshed = true
	return "secret_id_unrefreshed", "secret_key_unrefreshed", "token_unrefreshed", int(time.Now().Add(mrf.expiredDuration).Unix()), nil
}

func TestNormCredential(t *testing.T) {

	expiredDuration := time.Second * 2

	refresher := &mockRefresher{
		expiredDuration: expiredDuration,
	}

	cred, err := NewNormCredential(expiredDuration, refresher)

	if err != nil {
		t.Fatal(err)
	}

	secretId, err := cred.GetSecretId()
	if err != nil {
		t.Fatal(err)
	}

	secretKey, err := cred.GetSecretKey()
	if err != nil {
		t.Fatal(err)
	}

	if secretId != "secret_id_unrefreshed"{
		t.Fatalf("secret id %s mismatch", secretId)
	}

	if secretKey != "secret_key_unrefreshed"{
		t.Fatalf("secret key %s mismatch", secretKey)
	}

	time.Sleep(expiredDuration / 2)

	if cred.needRefresh() != true {
		t.Fatalf(
			"credential need to be refreshed before %s",
			cred.expiredAt.Add(-expiredDuration/2),
		)
	}

	secretId, err = cred.GetSecretId()
	if err != nil {
		t.Fatal(err)
	}

	secretKey, err = cred.GetSecretKey()
	if err != nil {
		t.Fatal(err)
	}

	if secretId != "secret_id_refreshed"{
		t.Fatalf("secret id %s mismatch", secretId)
	}

	if secretKey != "secret_key_refreshed"{
		t.Fatalf("secret key %s mismatch", secretKey)
	}

}
