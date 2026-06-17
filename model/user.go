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

// User 系统用户。
type User struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Username    string     `json:"username" gorm:"uniqueIndex"`
	Password    string     `json:"password,omitempty"`
	Email       string     `json:"email"`
	DisplayName string     `json:"displayName"`
	AvatarURL   string     `json:"avatarUrl"`
	Role        UserRole   `json:"role"`
	Credits     float64    `json:"credits"`
	GiftCredits float64    `json:"giftCredits"`
	AffCode     string     `json:"affCode" gorm:"uniqueIndex"`
	AffCount    int        `json:"affCount"`
	InviterID   string     `json:"inviterId"`
	GithubID    string     `json:"githubId"`
	LinuxDoID   string     `json:"linuxDoId" gorm:"index"`
	WechatID    string     `json:"wechatId"`
	Status      UserStatus `json:"status"`
	LastLoginAt string     `json:"lastLoginAt"`
	Extra       string     `json:"extra" gorm:"type:text"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
}

// UserList 用户分页结果。
type UserList struct {
	Items []User `json:"items"`
	Total int    `json:"total"`
}

// AuthUser 用户公开信息。
type AuthUser struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	AvatarURL   string   `json:"avatarUrl"`
	Role        UserRole `json:"role"`
	Credits     float64  `json:"credits"`
	GiftCredits float64  `json:"giftCredits"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// AuthSession 登录会话信息。
type AuthSession struct {
	Token string   `json:"token"`
	User  AuthUser `json:"user"`
}

func PublicUser(user User) AuthUser {
	return AuthUser{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
		Credits:     user.Credits,
		GiftCredits: user.GiftCredits,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

// CreditDebit 记录一次模型调用实际扣除的额度来源。
type CreditDebit struct {
	Credits     float64 `json:"credits"`
	GiftCredits float64 `json:"giftCredits"`
	Total       float64 `json:"total"`
}

type CreditLogType string

const (
	CreditLogTypeAdminAdjust  CreditLogType = "admin_adjust"
	CreditLogTypeAIConsume    CreditLogType = "ai_consume"
	CreditLogTypeAIRefund     CreditLogType = "ai_refund"
	CreditLogTypeRegisterGift CreditLogType = "register_gift"
)

// CreditLog 用户算力点变更流水。
type CreditLog struct {
	ID        string        `json:"id" gorm:"primaryKey"`
	UserID    string        `json:"userId" gorm:"index"`
	Type      CreditLogType `json:"type"`
	Amount    float64       `json:"amount"`
	Balance   float64       `json:"balance"`
	RelatedID string        `json:"relatedId"`
	Remark    string        `json:"remark"`
	Extra     string        `json:"extra" gorm:"type:text"`
	CreatedAt string        `json:"createdAt"`
}

type CreditLogList struct {
	Items []CreditLog `json:"items"`
	Total int         `json:"total"`
}
