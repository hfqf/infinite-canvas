package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Port                         string `env:"PORT" envDefault:"8080"`
	AdminUsername                string `env:"ADMIN_USERNAME" envDefault:"admin"`
	AdminPassword                string `env:"ADMIN_PASSWORD" envDefault:"infinite-canvas"`
	JWTSecret                    string `env:"JWT_SECRET" envDefault:"infinite-canvas"`
	JWTExpireHours               int    `env:"JWT_EXPIRE_HOURS" envDefault:"168"`
	DatabaseDriver               string `env:"DATABASE_DRIVER"`
	StorageDriver                string `env:"STORAGE_DRIVER" envDefault:"sqlite"`
	DatabaseDSN                  string `env:"DATABASE_DSN" envDefault:"data/infinite-canvas.db"`
	ImageStorageDriver           string `env:"FIGO_STORAGE_DRIVER" envDefault:"oss"`
	AliyunOSSEndpoint            string `env:"ALIYUN_OSS_ENDPOINT"`
	AliyunOSSBucket              string `env:"ALIYUN_OSS_BUCKET"`
	AliyunOSSAccessKeyID         string `env:"ALIYUN_OSS_ACCESS_KEY_ID"`
	AliyunOSSAccessKeySecret     string `env:"ALIYUN_OSS_ACCESS_KEY_SECRET"`
	AliyunOSSPublicBaseURL       string `env:"ALIYUN_OSS_PUBLIC_BASE_URL"`
	PublicBaseURL                string `env:"PUBLIC_BASE_URL"`
	Png2SVGCleanToolDir          string `env:"PNG2SVG_CLEAN_TOOL_DIR" envDefault:"png2svg-clean-node"`
	Png2SVGCleanNodePath         string `env:"PNG2SVG_CLEAN_NODE_PATH" envDefault:"node"`
	Png2SVGCleanProfile          string `env:"PNG2SVG_CLEAN_PROFILE" envDefault:"generic-clean-logo"`
	Png2SVGCleanTimeoutSec       int    `env:"PNG2SVG_CLEAN_TIMEOUT_SEC" envDefault:"90"`
	LinuxDoAuthorizeURL          string `env:"LINUX_DO_AUTHORIZE_URL" envDefault:"https://connect.linux.do/oauth2/authorize"`
	LinuxDoTokenURL              string `env:"LINUX_DO_TOKEN_URL" envDefault:"https://connect.linux.do/oauth2/token"`
	LinuxDoUserInfoURL           string `env:"LINUX_DO_USERINFO_URL" envDefault:"https://connect.linux.do/api/user"`
	WechatPayEnabled             bool   `env:"WECHAT_PAY_ENABLED" envDefault:"false"`
	WechatPayAppID               string `env:"WECHAT_PAY_APP_ID"`
	WechatPayMchID               string `env:"WECHAT_PAY_MCH_ID"`
	WechatPayAPIv3Secret         string `env:"WECHAT_PAY_API_V3_SECRET"`
	WechatPayKeyPath             string `env:"WECHAT_PAY_KEY_PATH"`
	WechatPayCertificateSerialNo string `env:"WECHAT_PAY_CERTIFICATE_SERIAL_NO"`
	WechatPayPublicKeyID         string `env:"WECHAT_PAY_PUBLIC_KEY_ID"`
	WechatPayPublicKeyPath       string `env:"WECHAT_PAY_PUBLIC_KEY_PATH"`
	WechatPaySkipNotifyVerify    bool   `env:"WECHAT_PAY_SKIP_NOTIFY_VERIFY" envDefault:"false"`
	WechatPayNotifyURL           string `env:"WECHAT_PAY_NOTIFY_URL"`
	VerificationProvider         string `env:"FIGO_VERIFICATION_PROVIDER" envDefault:"noop"`
	SMTPHost                     string `env:"FIGO_SMTP_HOST"`
	SMTPPort                     string `env:"FIGO_SMTP_PORT"`
	SMTPUsername                 string `env:"FIGO_SMTP_USERNAME"`
	SMTPPassword                 string `env:"FIGO_SMTP_PASSWORD"`
	SMTPFrom                     string `env:"FIGO_SMTP_FROM"`
}

var Cfg Config

func Load() error {
	_ = godotenv.Load()
	if err := env.Parse(&Cfg); err != nil {
		return err
	}
	Cfg.VerificationProvider = strings.ToLower(strings.TrimSpace(Cfg.VerificationProvider))
	if Cfg.VerificationProvider == "" {
		Cfg.VerificationProvider = "noop"
	}
	normalizeDatabaseDriver()
	normalizeDockerSQLiteDSN("/app/data")
	if strings.TrimSpace(Cfg.JWTSecret) == "" || Cfg.JWTSecret == "infinite-canvas" {
		secret, err := randomSecret()
		if err != nil {
			return err
		}
		Cfg.JWTSecret = secret
	}
	return nil
}

func normalizeDatabaseDriver() {
	driver := strings.ToLower(strings.TrimSpace(Cfg.DatabaseDriver))
	if driver == "" {
		driver = strings.ToLower(strings.TrimSpace(Cfg.StorageDriver))
	}
	if driver == "" {
		driver = "sqlite"
	}
	Cfg.DatabaseDriver = driver
	Cfg.StorageDriver = driver
}

func normalizeDockerSQLiteDSN(appDataDir string) {
	driver := strings.ToLower(strings.TrimSpace(Cfg.StorageDriver))
	if driver != "" && driver != "sqlite" {
		return
	}
	dsn := strings.TrimSpace(Cfg.DatabaseDSN)
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return
	}
	pathPart, suffix := dsn, ""
	if index := strings.Index(dsn, "?"); index >= 0 {
		pathPart = dsn[:index]
		suffix = dsn[index:]
	}
	if filepath.IsAbs(pathPart) {
		return
	}
	slashPath := filepath.ToSlash(pathPart)
	if slashPath != "data" && !strings.HasPrefix(slashPath, "data/") {
		return
	}
	if _, err := os.Stat(appDataDir); err != nil {
		return
	}
	Cfg.DatabaseDSN = filepath.Join(filepath.Dir(appDataDir), filepath.FromSlash(slashPath)) + suffix
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
