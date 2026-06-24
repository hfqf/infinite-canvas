package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

const (
	verificationTTL           = 10 * time.Minute
	verificationIssueInterval = 60 * time.Second
	verificationMaxAttempts   = 5
)

type VerificationIssueResult struct {
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	DebugCode        string `json:"debugCode,omitempty"`
}

type verificationProvider interface {
	SendVerificationCode(email, purpose, code string, expiresAt time.Time) error
}

type noopVerificationProvider struct{}

func (noopVerificationProvider) SendVerificationCode(string, string, string, time.Time) error {
	return nil
}

type smtpVerificationProvider struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func RequestVerificationCode(email string, purpose string) (VerificationIssueResult, error) {
	email, err := normalizeVerificationEmail(email)
	if err != nil {
		return VerificationIssueResult{}, err
	}
	purpose, err = normalizeVerificationPurpose(purpose)
	if err != nil {
		return VerificationIssueResult{}, err
	}
	if purpose == model.VerificationPurposeRegister {
		if _, ok, err := repository.GetUserByEmail(email); err != nil {
			return VerificationIssueResult{}, err
		} else if ok {
			return VerificationIssueResult{}, safeMessageError{message: "邮箱已注册"}
		}
	}
	if latest, ok, err := repository.LatestEmailVerificationCode(email, purpose); err != nil {
		return VerificationIssueResult{}, err
	} else if ok {
		createdAt, parseErr := time.Parse(time.RFC3339, latest.CreatedAt)
		expiresAt, expiresErr := time.Parse(time.RFC3339, latest.ExpiresAt)
		if parseErr == nil && expiresErr == nil && time.Since(createdAt) < verificationIssueInterval && time.Now().Before(expiresAt) {
			return VerificationIssueResult{}, safeMessageError{message: "验证码获取太频繁，请稍后再试"}
		}
	}

	code, err := generateNumericVerificationCode(6)
	if err != nil {
		return VerificationIssueResult{}, err
	}
	salt := randomString(32)
	expiresAt := time.Now().Add(verificationTTL)
	record := model.EmailVerificationCode{
		ID:        newID("verify"),
		Email:     email,
		Purpose:   purpose,
		CodeHash:  hashVerificationCode(code, salt),
		CodeSalt:  salt,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		CreatedAt: now(),
	}
	if _, err = repository.SaveEmailVerificationCode(record); err != nil {
		return VerificationIssueResult{}, err
	}
	provider, exposeDebugCode, err := buildVerificationProvider()
	if err != nil {
		_ = repository.DeleteEmailVerificationCode(record.ID)
		return VerificationIssueResult{}, err
	}
	if err = provider.SendVerificationCode(email, purpose, code, expiresAt); err != nil {
		_ = repository.DeleteEmailVerificationCode(record.ID)
		return VerificationIssueResult{}, err
	}
	result := VerificationIssueResult{ExpiresInSeconds: int(verificationTTL.Seconds())}
	if exposeDebugCode {
		result.DebugCode = code
	}
	return result, nil
}

func consumeVerificationCode(email string, purpose string, code string) error {
	email, err := normalizeVerificationEmail(email)
	if err != nil {
		return err
	}
	purpose, err = normalizeVerificationPurpose(purpose)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return safeMessageError{message: "请输入邮箱验证码"}
	}
	record, ok, err := repository.LatestEmailVerificationCode(email, purpose)
	if err != nil {
		return err
	}
	if !ok {
		return safeMessageError{message: "邮箱验证码无效或已过期"}
	}
	expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt)
	if err != nil || !time.Now().Before(expiresAt) || record.Attempts >= verificationMaxAttempts {
		return safeMessageError{message: "邮箱验证码无效或已过期"}
	}
	record.Attempts += 1
	if subtle.ConstantTimeCompare([]byte(record.CodeHash), []byte(hashVerificationCode(code, record.CodeSalt))) != 1 {
		_, _ = repository.SaveEmailVerificationCode(record)
		return safeMessageError{message: "邮箱验证码错误"}
	}
	record.ConsumedAt = now()
	_, err = repository.SaveEmailVerificationCode(record)
	return err
}

func normalizeVerificationEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", safeMessageError{message: "邮箱不能为空"}
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", safeMessageError{message: "邮箱格式不正确"}
	}
	return value, nil
}

func normalizeVerificationPurpose(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = model.VerificationPurposeRegister
	}
	if value != model.VerificationPurposeRegister {
		return "", safeMessageError{message: "验证码用途不支持"}
	}
	return value, nil
}

func generateNumericVerificationCode(length int) (string, error) {
	var builder strings.Builder
	builder.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}

func hashVerificationCode(code string, salt string) string {
	sum := sha256.Sum256([]byte(salt + "\x00" + strings.TrimSpace(code)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func buildVerificationProvider() (verificationProvider, bool, error) {
	switch strings.ToLower(strings.TrimSpace(config.Cfg.VerificationProvider)) {
	case "", "noop":
		return noopVerificationProvider{}, true, nil
	case "smtp":
		provider := smtpVerificationProvider{
			host:     strings.TrimSpace(config.Cfg.SMTPHost),
			port:     strings.TrimSpace(config.Cfg.SMTPPort),
			username: strings.TrimSpace(config.Cfg.SMTPUsername),
			password: strings.TrimSpace(config.Cfg.SMTPPassword),
			from:     strings.TrimSpace(config.Cfg.SMTPFrom),
		}
		if provider.host == "" || provider.port == "" || provider.username == "" || provider.password == "" || provider.from == "" {
			return nil, false, safeMessageError{message: "邮箱验证码 SMTP 配置不完整"}
		}
		return provider, false, nil
	default:
		return nil, false, safeMessageError{message: "邮箱验证码服务商配置不支持"}
	}
}

func (provider smtpVerificationProvider) SendVerificationCode(email, purpose, code string, expiresAt time.Time) error {
	subject := "好图秀注册验证码"
	minutes := int(time.Until(expiresAt).Round(time.Minute).Minutes())
	if minutes < 1 {
		minutes = 1
	}
	body := fmt.Sprintf("您好，您正在注册好图秀。\n\n您的好图秀注册验证码是：%s。\n\n验证码将在 %d 分钟后过期。为了保障账号安全，请勿将验证码告知他人。\n\n如果这不是您本人操作，请忽略本邮件。\n\n好图秀团队\nwww.haotushow.com", code, minutes)
	message := buildSMTPMessage(provider.from, email, subject, body)
	auth := smtp.PlainAuth("", provider.username, provider.password, provider.host)
	address := net.JoinHostPort(provider.host, provider.port)
	if provider.port == "465" {
		return smtpSendMailImplicitTLS(address, auth, provider.from, []string{email}, message)
	}
	return smtp.SendMail(address, auth, provider.from, []string{email}, message)
}

func buildSMTPMessage(from string, to string, subject string, body string) []byte {
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n")
}

func smtpSendMailImplicitTLS(address string, auth smtp.Auth, from string, to []string, message []byte) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	conn, err := tls.Dial("tcp", address, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("smtp tls 连接失败: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Quit()
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("smtp server does not support AUTH")
		}
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}
