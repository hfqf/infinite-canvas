package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

func TestGetRechargeOrderRefreshesPaidWechatTransaction(t *testing.T) {
	previousConfig := config.Cfg
	previousWechatPayAPIBaseURL := wechatPayAPIBaseURL
	t.Cleanup(func() {
		config.Cfg = previousConfig
		wechatPayAPIBaseURL = previousWechatPayAPIBaseURL
	})
	config.Cfg = config.Config{
		StorageDriver:                "sqlite",
		DatabaseDSN:                  filepath.Join(t.TempDir(), "test.db"),
		WechatPayMchID:               "mch_test",
		WechatPayCertificateSerialNo: "serial_test",
		WechatPayKeyPath:             writeTestWechatPayPrivateKey(t),
	}

	user, err := repository.SaveUser(model.User{ID: "user_recharge_sync_001", Username: "recharge-sync-user", Role: model.UserRoleUser, Status: model.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	order, err := model.NewRechargeOrderByAmountFen(user.ID, 10, "created")
	if err != nil {
		t.Fatal(err)
	}
	order.ID = "recharge_sync_order_001"
	order.OutTradeNo = "trade_sync_001"
	order.CodeURL = "weixin://pay/test"
	if order, err = repository.SaveRechargeOrder(order); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/pay/transactions/out-trade-no/trade_sync_001" {
			t.Fatalf("query path=%s, want trade_sync_001 transaction query", r.URL.Path)
		}
		if r.URL.Query().Get("mchid") != "mch_test" {
			t.Fatalf("mchid=%s, want mch_test", r.URL.Query().Get("mchid"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"out_trade_no":"trade_sync_001","transaction_id":"wx_tx_sync_001","trade_state":"SUCCESS","success_time":"paid-time","amount":{"total":10}}`))
	}))
	defer server.Close()
	wechatPayAPIBaseURL = server.URL

	result, err := GetRechargeOrder(user.ID, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != model.RechargeOrderStatusPaid || result.PaidAt != "paid-time" {
		t.Fatalf("result status=%s paidAt=%s, want paid/paid-time", result.Status, result.PaidAt)
	}
	user, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 1 || user.MemberType != model.MemberTypeTest || user.MemberLevel != model.MemberLevelTest || user.LastRechargeAmountYuan != 0 || user.LastRechargedAt != "paid-time" {
		t.Fatalf("user recharge fields=%#v, want 1 credit and test membership", user)
	}
}

func writeTestWechatPayPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "apiclient_key.pem")
	value := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, value, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
