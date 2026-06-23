package service

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

type RechargeOrderResult struct {
	ID          string                    `json:"id"`
	AmountYuan  int                       `json:"amountYuan"`
	AmountFen   int                       `json:"amountFen"`
	Credits     int                       `json:"credits"`
	MemberType  model.MemberType          `json:"memberType"`
	MemberLevel model.MemberLevel         `json:"memberLevel"`
	ProductName string                    `json:"productName"`
	Status      model.RechargeOrderStatus `json:"status"`
	CodeURL     string                    `json:"codeUrl"`
	CreatedAt   string                    `json:"createdAt"`
	PaidAt      string                    `json:"paidAt"`
}

type wechatNativeOrderResponse struct {
	CodeURL string `json:"code_url"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type wechatPayNotify struct {
	EventType string `json:"event_type"`
	Resource  struct {
		Algorithm      string `json:"algorithm"`
		Ciphertext     string `json:"ciphertext"`
		AssociatedData string `json:"associated_data"`
		Nonce          string `json:"nonce"`
	} `json:"resource"`
}

type wechatPayTransaction struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	SuccessTime   string `json:"success_time"`
	Amount        struct {
		Total int `json:"total"`
	} `json:"amount"`
}

func CreateRechargeOrder(userID string, amountYuan int, notifyURL string) (RechargeOrderResult, error) {
	if !config.Cfg.WechatPayEnabled {
		return RechargeOrderResult{}, safeMessageError{message: "微信支付暂未开启"}
	}
	user, ok, err := repository.GetUserByID(userID)
	if err != nil {
		return RechargeOrderResult{}, err
	}
	if !ok {
		return RechargeOrderResult{}, safeMessageError{message: "用户不存在"}
	}
	order, err := model.NewRechargeOrder(user.ID, amountYuan, now())
	if err != nil {
		return RechargeOrderResult{}, safeMessageError{message: err.Error()}
	}
	order.ID = newID("recharge")
	order.OutTradeNo = newRechargeTradeNo()
	order, err = repository.SaveRechargeOrder(order)
	if err != nil {
		return RechargeOrderResult{}, err
	}
	codeURL, err := createWechatNativeOrder(order, notifyURL)
	if err != nil {
		return RechargeOrderResult{}, err
	}
	order.CodeURL = codeURL
	order.UpdatedAt = now()
	order, err = repository.SaveRechargeOrder(order)
	if err != nil {
		return RechargeOrderResult{}, err
	}
	return rechargeOrderResult(order), nil
}

func GetRechargeOrder(userID string, id string) (RechargeOrderResult, error) {
	order, ok, err := repository.GetRechargeOrderByID(id)
	if err != nil {
		return RechargeOrderResult{}, err
	}
	if !ok || order.UserID != userID {
		return RechargeOrderResult{}, safeMessageError{message: "订单不存在"}
	}
	return rechargeOrderResult(order), nil
}

func HandleWechatRechargeNotify(r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := verifyWechatPayNotifySignature(r, body); err != nil {
		return err
	}
	var notify wechatPayNotify
	if err := json.Unmarshal(body, &notify); err != nil {
		return err
	}
	if notify.EventType != "TRANSACTION.SUCCESS" {
		return nil
	}
	plain, err := decryptWechatPayResource(notify)
	if err != nil {
		return err
	}
	var transaction wechatPayTransaction
	if err := json.Unmarshal(plain, &transaction); err != nil {
		return err
	}
	if transaction.TradeState != "SUCCESS" {
		return nil
	}
	_, err = repository.CompleteRechargeOrderPaid(transaction.OutTradeNo, transaction.Amount.Total, transaction.TransactionID, firstNonEmpty(transaction.SuccessTime, now()))
	return err
}

func createWechatNativeOrder(order model.RechargeOrder, notifyURL string) (string, error) {
	if strings.TrimSpace(config.Cfg.WechatPayAppID) == "" || strings.TrimSpace(config.Cfg.WechatPayMchID) == "" || strings.TrimSpace(config.Cfg.WechatPayCertificateSerialNo) == "" || strings.TrimSpace(config.Cfg.WechatPayKeyPath) == "" {
		return "", safeMessageError{message: "微信支付配置不完整"}
	}
	if strings.TrimSpace(notifyURL) == "" {
		return "", safeMessageError{message: "微信支付回调地址未配置"}
	}
	body, _ := json.Marshal(map[string]any{
		"appid":        config.Cfg.WechatPayAppID,
		"mchid":        config.Cfg.WechatPayMchID,
		"description":  order.ProductName,
		"out_trade_no": order.OutTradeNo,
		"notify_url":   notifyURL,
		"amount": map[string]any{
			"total":    order.AmountFen,
			"currency": "CNY",
		},
	})
	request, err := http.NewRequest(http.MethodPost, "https://api.mch.weixin.qq.com/v3/pay/transactions/native", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Infinite-Canvas/1.0")
	if err := signWechatPayRequest(request, body); err != nil {
		return "", err
	}
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	var result wechatNativeOrderResponse
	_ = json.Unmarshal(responseBody, &result)
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.CodeURL == "" {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "微信支付下单失败"
		}
		return "", safeMessageError{message: message}
	}
	return result.CodeURL, nil
}

func signWechatPayRequest(request *http.Request, body []byte) error {
	privateKey, err := loadRSAPrivateKey(config.Cfg.WechatPayKeyPath)
	if err != nil {
		return err
	}
	nonce := randomString(32)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	path := request.URL.RequestURI()
	message := request.Method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return err
	}
	authorization := `WECHATPAY2-SHA256-RSA2048 mchid="` + config.Cfg.WechatPayMchID + `",nonce_str="` + nonce + `",timestamp="` + timestamp + `",serial_no="` + config.Cfg.WechatPayCertificateSerialNo + `",signature="` + base64.StdEncoding.EncodeToString(signature) + `"`
	request.Header.Set("Authorization", authorization)
	return nil
}

func verifyWechatPayNotifySignature(request *http.Request, body []byte) error {
	if config.Cfg.WechatPaySkipNotifyVerify {
		return nil
	}
	publicKeyPath := strings.TrimSpace(config.Cfg.WechatPayPublicKeyPath)
	if publicKeyPath == "" {
		return safeMessageError{message: "微信支付平台公钥未配置"}
	}
	publicKey, err := loadRSAPublicKey(publicKeyPath)
	if err != nil {
		return err
	}
	timestamp := request.Header.Get("Wechatpay-Timestamp")
	nonce := request.Header.Get("Wechatpay-Nonce")
	signatureValue := request.Header.Get("Wechatpay-Signature")
	serial := request.Header.Get("Wechatpay-Serial")
	if timestamp == "" || nonce == "" || signatureValue == "" {
		return errors.New("微信支付通知签名头缺失")
	}
	if publicKeyID := strings.TrimSpace(config.Cfg.WechatPayPublicKeyID); publicKeyID != "" {
		if serial == "" || serial != publicKeyID {
			return errors.New("微信支付平台公钥 ID 不匹配")
		}
	}
	signature, err := base64.StdEncoding.DecodeString(signatureValue)
	if err != nil {
		return err
	}
	message := timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	hash := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature)
}

func decryptWechatPayResource(notify wechatPayNotify) ([]byte, error) {
	key := []byte(config.Cfg.WechatPayAPIv3Secret)
	if len(key) != 32 {
		return nil, safeMessageError{message: "微信支付 API v3 Secret 必须为 32 字节"}
	}
	ciphertext, err := base64.StdEncoding.DecodeString(notify.Resource.Ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(notify.Resource.Nonce), ciphertext, []byte(notify.Resource.AssociatedData))
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(value)
	if block == nil {
		return nil, errors.New("无法解析商户私钥")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("商户私钥不是 RSA 私钥")
	}
	return key, nil
}

func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(value)
	if block == nil {
		return nil, errors.New("无法解析微信支付平台公钥")
	}
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if key, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		key, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes)
		if pkcs1Err == nil {
			return key, nil
		}
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("微信支付平台公钥不是 RSA 公钥")
	}
	return key, nil
}

func rechargeOrderResult(order model.RechargeOrder) RechargeOrderResult {
	return RechargeOrderResult{ID: order.ID, AmountYuan: order.AmountYuan, AmountFen: order.AmountFen, Credits: order.Credits, MemberType: order.MemberType, MemberLevel: order.MemberLevel, ProductName: order.ProductName, Status: order.Status, CodeURL: order.CodeURL, CreatedAt: order.CreatedAt, PaidAt: order.PaidAt}
}

func newRechargeTradeNo() string {
	return "rcg" + strconv.FormatInt(time.Now().UnixNano(), 36) + randomString(6)
}

func randomString(length int) string {
	const letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	for i := range buf {
		buf[i] = letters[int(buf[i])%len(letters)]
	}
	return string(buf)
}
