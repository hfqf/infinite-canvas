package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/service"
)

type createRechargeOrderRequest struct {
	AmountYuan int `json:"amountYuan"`
	AmountFen  int `json:"amountFen"`
}

func CreateRechargeOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		Fail(w, "请先登录")
		return
	}
	var request createRechargeOrderRequest
	_ = json.NewDecoder(r.Body).Decode(&request)
	amountFen := request.AmountFen
	if amountFen <= 0 {
		amountFen = request.AmountYuan * 100
	}
	order, err := service.CreateRechargeOrder(user.ID, amountFen, rechargeNotifyURL(r))
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, order)
}

func RechargeOrder(w http.ResponseWriter, r *http.Request, id string) {
	user, ok := service.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		Fail(w, "请先登录")
		return
	}
	order, err := service.GetRechargeOrder(user.ID, id)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, order)
}

func WechatRechargeNotify(w http.ResponseWriter, r *http.Request) {
	if err := service.HandleWechatRechargeNotify(r); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "FAIL", "message": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"code": "SUCCESS", "message": "成功"})
}

func rechargeNotifyURL(r *http.Request) string {
	if url := strings.TrimSpace(config.Cfg.WechatPayNotifyURL); url != "" {
		return url
	}
	base := strings.TrimRight(strings.TrimSpace(config.Cfg.PublicBaseURL), "/")
	if base == "" {
		host := r.Header.Get("x-forwarded-host")
		if host == "" {
			host = r.Host
		}
		proto := r.Header.Get("x-forwarded-proto")
		if proto == "" {
			proto = "http"
		}
		base = proto + "://" + host
	}
	return base + "/api/recharge/wechat/notify"
}
