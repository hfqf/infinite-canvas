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

type MemberType string

const (
	MemberTypeMonthly MemberType = "monthly"
	MemberTypeAnnual  MemberType = "annual"
)

type MemberLevel string

const (
	MemberLevelStandard MemberLevel = "standard"
	MemberLevelBasic    MemberLevel = "basic"
	MemberLevelAdvanced MemberLevel = "advanced"
	MemberLevelPremium  MemberLevel = "premium"
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
	DisplayName            string      `json:"displayName"`
	AvatarURL              string      `json:"avatarUrl"`
	Role                   UserRole    `json:"role"`
	Credits                int         `json:"credits"`
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
		DisplayName:            user.DisplayName,
		AvatarURL:              user.AvatarURL,
		Role:                   user.Role,
		Credits:                user.Credits,
		MemberType:             user.MemberType,
		MemberLevel:            user.MemberLevel,
		LastRechargeAmountYuan: user.LastRechargeAmountYuan,
		LastRechargedAt:        user.LastRechargedAt,
		CreatedAt:              user.CreatedAt,
		UpdatedAt:              user.UpdatedAt,
	}
}

type CreditLogType string

const (
	CreditLogTypeAdminAdjust  CreditLogType = "admin_adjust"
	CreditLogTypeAIConsume    CreditLogType = "ai_consume"
	CreditLogTypeAIRefund     CreditLogType = "ai_refund"
	CreditLogTypeRegisterGift CreditLogType = "register_gift"
	CreditLogTypeRecharge     CreditLogType = "recharge"
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

type RechargeOrderStatus string

const (
	RechargeOrderStatusPending RechargeOrderStatus = "pending"
	RechargeOrderStatusPaid    RechargeOrderStatus = "paid"
)

const (
	RechargePaymentMethodWechat = "wechat"
)

type RechargePlan struct {
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
	plan, ok := RechargePlanForAmount(amountYuan)
	if !ok {
		return RechargeOrder{}, validationError("请选择有效充值套餐")
	}
	return RechargeOrder{
		UserID:        userID,
		AmountYuan:    amountYuan,
		AmountFen:     amountYuan * 100,
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
	for _, plan := range rechargePlans() {
		if plan.AmountYuan == amountYuan {
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
	}
}

func newRechargePlan(amountYuan int, credits int, memberType MemberType, memberLevel MemberLevel, typeName string, levelName string) RechargePlan {
	return RechargePlan{
		AmountYuan:  amountYuan,
		Credits:     credits,
		MemberType:  memberType,
		MemberLevel: memberLevel,
		TypeName:    typeName,
		LevelName:   levelName,
		ProductName: "好图秀AI算力充值-" + typeName + "-" + levelName,
	}
}

type validationError string

func (err validationError) Error() string {
	return string(err)
}
