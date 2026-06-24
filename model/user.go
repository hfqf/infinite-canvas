package model

type UserRole string

const (
	UserRoleGuest UserRole = "guest"
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

type UserStatus string

const (
	UserStatusActive UserStatus = "active"
	UserStatusBan    UserStatus = "ban"
)

const VerificationPurposeRegister = "register"

type MemberType string

const (
	MemberTypeMonthly MemberType = "monthly"
	MemberTypeAnnual  MemberType = "annual"
	MemberTypeTest    MemberType = "test"
)

type MemberLevel string

const (
	MemberLevelStandard MemberLevel = "standard"
	MemberLevelBasic    MemberLevel = "basic"
	MemberLevelAdvanced MemberLevel = "advanced"
	MemberLevelPremium  MemberLevel = "premium"
	MemberLevelTest     MemberLevel = "test"
)

// User 系统用户。
type User struct {
	ID                     string      `json:"id" gorm:"primaryKey"`
	Username               string      `json:"username" gorm:"uniqueIndex"`
	Password               string      `json:"password,omitempty"`
	Email                  string      `json:"email"`
	DisplayName            string      `json:"displayName"`
	AvatarURL              string      `json:"avatarUrl"`
	Role                   UserRole    `json:"role"`
	Credits                int         `json:"credits"`
	FrozenCredits          int         `json:"frozenCredits"`
	MemberType             MemberType  `json:"memberType"`
	MemberLevel            MemberLevel `json:"memberLevel"`
	LastRechargeAmountYuan int         `json:"lastRechargeAmountYuan"`
	LastRechargedAt        string      `json:"lastRechargedAt"`
	AffCode                string      `json:"affCode" gorm:"uniqueIndex"`
	AffCount               int         `json:"affCount"`
	InviterID              string      `json:"inviterId"`
	GithubID               string      `json:"githubId"`
	LinuxDoID              string      `json:"linuxDoId" gorm:"index"`
	WechatID               string      `json:"wechatId"`
	Status                 UserStatus  `json:"status"`
	LastLoginAt            string      `json:"lastLoginAt"`
	Extra                  string      `json:"extra" gorm:"type:text"`
	CreatedAt              string      `json:"createdAt"`
	UpdatedAt              string      `json:"updatedAt"`
}

// UserList 用户分页结果。
type UserList struct {
	Items []User `json:"items"`
	Total int    `json:"total"`
}

// AuthUser 用户公开信息。
type AuthUser struct {
	ID                     string      `json:"id"`
	Username               string      `json:"username"`
	Email                  string      `json:"email"`
	DisplayName            string      `json:"displayName"`
	AvatarURL              string      `json:"avatarUrl"`
	Role                   UserRole    `json:"role"`
	Credits                int         `json:"credits"`
	FrozenCredits          int         `json:"frozenCredits"`
	MemberType             MemberType  `json:"memberType"`
	MemberLevel            MemberLevel `json:"memberLevel"`
	LastRechargeAmountYuan int         `json:"lastRechargeAmountYuan"`
	LastRechargedAt        string      `json:"lastRechargedAt"`
	CreatedAt              string      `json:"createdAt"`
	UpdatedAt              string      `json:"updatedAt"`
}

// AuthSession 登录会话信息。
type AuthSession struct {
	Token string   `json:"token"`
	User  AuthUser `json:"user"`
}

func PublicUser(user User) AuthUser {
	return AuthUser{
		ID:                     user.ID,
		Username:               user.Username,
		Email:                  user.Email,
		DisplayName:            user.DisplayName,
		AvatarURL:              user.AvatarURL,
		Role:                   user.Role,
		Credits:                user.Credits,
		FrozenCredits:          user.FrozenCredits,
		MemberType:             user.MemberType,
		MemberLevel:            user.MemberLevel,
		LastRechargeAmountYuan: user.LastRechargeAmountYuan,
		LastRechargedAt:        user.LastRechargedAt,
		CreatedAt:              user.CreatedAt,
		UpdatedAt:              user.UpdatedAt,
	}
}

// EmailVerificationCode 邮箱验证码记录。
type EmailVerificationCode struct {
	ID         string `json:"id" gorm:"primaryKey"`
	Email      string `json:"email" gorm:"index"`
	Purpose    string `json:"purpose" gorm:"index"`
	CodeHash   string `json:"codeHash"`
	CodeSalt   string `json:"codeSalt"`
	ExpiresAt  string `json:"expiresAt"`
	ConsumedAt string `json:"consumedAt"`
	Attempts   int    `json:"attempts"`
	CreatedAt  string `json:"createdAt"`
}

type CreditLogType string

const (
	CreditLogTypeAdminAdjust     CreditLogType = "admin_adjust"
	CreditLogTypeAIFreeze        CreditLogType = "ai_freeze"
	CreditLogTypeAIFreezeRelease CreditLogType = "ai_freeze_release"
	CreditLogTypeAIConsume       CreditLogType = "ai_consume"
	CreditLogTypeAIRefund        CreditLogType = "ai_refund"
	CreditLogTypeRegisterGift    CreditLogType = "register_gift"
	CreditLogTypeRecharge        CreditLogType = "recharge"
)

// CreditLog 用户算力点变更流水。
type CreditLog struct {
	ID        string        `json:"id" gorm:"primaryKey"`
	UserID    string        `json:"userId" gorm:"index"`
	Type      CreditLogType `json:"type"`
	Amount    int           `json:"amount"`
	Balance   int           `json:"balance"`
	RelatedID string        `json:"relatedId"`
	Remark    string        `json:"remark"`
	Extra     string        `json:"extra" gorm:"type:text"`
	CreatedAt string        `json:"createdAt"`
}

type CreditLogList struct {
	Items []CreditLog `json:"items"`
	Total int         `json:"total"`
}

// AIImageTask 保存异步生图任务的计费上下文。
type AIImageTask struct {
	ID          string `json:"id" gorm:"primaryKey"`
	TaskID      string `json:"taskId" gorm:"uniqueIndex"`
	UserID      string `json:"userId" gorm:"index"`
	Model       string `json:"model"`
	Path        string `json:"path"`
	Prompt      string `json:"prompt" gorm:"type:text"`
	Credits     int    `json:"credits"`
	Status      string `json:"status" gorm:"index"`
	ImageURL    string `json:"imageUrl" gorm:"type:text"`
	ChannelName string `json:"channelName"`
	ChannelURL  string `json:"channelUrl"`
	FrozenAt    string `json:"frozenAt"`
	ChargedAt   string `json:"chargedAt"`
	ReleasedAt  string `json:"releasedAt"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type RechargeOrderStatus string

const (
	RechargeOrderStatusPending RechargeOrderStatus = "pending"
	RechargeOrderStatusPaid    RechargeOrderStatus = "paid"
)

const (
	RechargePaymentMethodWechat = "wechat"
)

type RechargePlan struct {
	AmountFen   int
	AmountYuan  int
	Credits     int
	MemberType  MemberType
	MemberLevel MemberLevel
	TypeName    string
	LevelName   string
	ProductName string
}

// RechargeOrder 用户充值订单。
type RechargeOrder struct {
	ID            string              `json:"id" gorm:"primaryKey"`
	UserID        string              `json:"userId" gorm:"index"`
	AmountYuan    int                 `json:"amountYuan"`
	AmountFen     int                 `json:"amountFen"`
	Credits       int                 `json:"credits"`
	MemberType    MemberType          `json:"memberType"`
	MemberLevel   MemberLevel         `json:"memberLevel"`
	ProductName   string              `json:"productName"`
	Status        RechargeOrderStatus `json:"status" gorm:"index"`
	PaymentMethod string              `json:"paymentMethod"`
	OutTradeNo    string              `json:"outTradeNo" gorm:"uniqueIndex"`
	TransactionID string              `json:"transactionId" gorm:"index"`
	CodeURL       string              `json:"codeUrl" gorm:"type:text"`
	PaidAt        string              `json:"paidAt"`
	CreatedAt     string              `json:"createdAt"`
	UpdatedAt     string              `json:"updatedAt"`
}

func NewRechargeOrder(userID string, amountYuan int, now string) (RechargeOrder, error) {
	return NewRechargeOrderByAmountFen(userID, amountYuan*100, now)
}

func NewRechargeOrderByAmountFen(userID string, amountFen int, now string) (RechargeOrder, error) {
	plan, ok := RechargePlanForAmountFen(amountFen)
	if !ok {
		return RechargeOrder{}, validationError("请选择有效充值套餐")
	}
	return RechargeOrder{
		UserID:        userID,
		AmountYuan:    plan.AmountYuan,
		AmountFen:     plan.AmountFen,
		Credits:       plan.Credits,
		MemberType:    plan.MemberType,
		MemberLevel:   plan.MemberLevel,
		ProductName:   plan.ProductName,
		Status:        RechargeOrderStatusPending,
		PaymentMethod: RechargePaymentMethodWechat,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func RechargePlanForAmount(amountYuan int) (RechargePlan, bool) {
	return RechargePlanForAmountFen(amountYuan * 100)
}

func RechargePlanForAmountFen(amountFen int) (RechargePlan, bool) {
	for _, plan := range rechargePlans() {
		if plan.AmountFen == amountFen {
			return plan, true
		}
	}
	return RechargePlan{}, false
}

func rechargePlans() []RechargePlan {
	return []RechargePlan{
		newRechargePlan(59, 590, MemberTypeMonthly, MemberLevelBasic, "月度", "基础版"),
		newRechargePlan(99, 1100, MemberTypeMonthly, MemberLevelAdvanced, "月度", "高级版"),
		newRechargePlan(199, 2488, MemberTypeMonthly, MemberLevelPremium, "月度", "尊享版"),
		newRechargePlan(499, 5000, MemberTypeAnnual, MemberLevelStandard, "年度", "普通版"),
		newRechargePlan(699, 6996, MemberTypeAnnual, MemberLevelBasic, "年度", "基础版"),
		newRechargePlan(999, 10020, MemberTypeAnnual, MemberLevelAdvanced, "年度", "高级版"),
		newRechargePlan(1999, 21044, MemberTypeAnnual, MemberLevelPremium, "年度", "尊享版"),
		newTestRechargePlan(10),
		newTestRechargePlan(20),
		newTestRechargePlan(30),
		newTestRechargePlan(40),
		newTestRechargePlan(50),
	}
}

func newRechargePlan(amountYuan int, credits int, memberType MemberType, memberLevel MemberLevel, typeName string, levelName string) RechargePlan {
	return RechargePlan{
		AmountFen:   amountYuan * 100,
		AmountYuan:  amountYuan,
		Credits:     credits,
		MemberType:  memberType,
		MemberLevel: memberLevel,
		TypeName:    typeName,
		LevelName:   levelName,
		ProductName: "好图秀AI算力充值-" + typeName + "-" + levelName,
	}
}

func newTestRechargePlan(amountFen int) RechargePlan {
	return RechargePlan{
		AmountFen:   amountFen,
		AmountYuan:  0,
		Credits:     amountFen / 10,
		MemberType:  MemberTypeTest,
		MemberLevel: MemberLevelTest,
		TypeName:    "测试",
		LevelName:   amountFenLabel(amountFen),
		ProductName: "好图秀AI算力充值-测试-" + amountFenLabel(amountFen),
	}
}

func amountFenLabel(amountFen int) string {
	yuan := amountFen / 100
	fen := amountFen % 100
	return string(rune('0'+yuan)) + "." + string(rune('0'+fen/10)) + string(rune('0'+fen%10)) + "元"
}

type validationError string

func (err validationError) Error() string {
	return string(err)
}
