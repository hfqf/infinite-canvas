package model

// Prompt 提示词记录。
type Prompt struct {
	ID        string   `json:"id" gorm:"primaryKey"`
	Title     string   `json:"title"`
	CoverURL  string   `json:"coverUrl"`
	Prompt    string   `json:"prompt"`
	Tags      []string `json:"tags" gorm:"serializer:json"`
	Category  string   `json:"category" gorm:"index"`
	GithubURL string   `json:"githubUrl" gorm:"-"`
	Preview   string   `json:"preview"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// PromptList 提示词分页结果。
type PromptList struct {
	Items      []Prompt `json:"items"`
	Tags       []string `json:"tags"`
	Categories []string `json:"categories"`
	Total      int      `json:"total"`
}

// UserPrompt 用户个人提示词记录。
type UserPrompt struct {
	ID        string   `json:"id" gorm:"primaryKey"`
	UserID    string   `json:"userId" gorm:"index"`
	Title     string   `json:"title"`
	Prompt    string   `json:"prompt"`
	Tags      []string `json:"tags" gorm:"serializer:json"`
	Category  string   `json:"category" gorm:"index"`
	SortOrder int      `json:"sortOrder" gorm:"index"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// UserPromptList 用户个人提示词分页结果。
type UserPromptList struct {
	Items      []UserPrompt `json:"items"`
	Tags       []string     `json:"tags"`
	Categories []string     `json:"categories"`
	Total      int          `json:"total"`
}

// UserPromptReorderItem 用户提示词排序项。
type UserPromptReorderItem struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sortOrder"`
}

// PromptCategory 提示词分类。
type PromptCategory struct {
	Category    string `json:"category" gorm:"primaryKey"`
	Name        string `json:"name"`
	Description string `json:"description"`
	GithubURL   string `json:"githubUrl"`
	Remote      bool   `json:"remote"`
	UpdatedAt   string `json:"updatedAt"`
}
