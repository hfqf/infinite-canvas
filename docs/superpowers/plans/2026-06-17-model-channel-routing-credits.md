# 模型渠道路由与赠送额度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use $superpower-subagents (recommended) or $superpower-executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking via update_plan.

**Goal:** 将模型配置升级为“公开模型规格、私有渠道路由、通用额度与赠送额度分账”的体系，使 `gpt-image-2-0.5k` 可作为系统可开关规格并且只消耗赠送额度，`gpt-image-2-1k/2k/4k` 可由多个中转服务商按权重和单独价格提供服务。

**Architecture:** 前端只展示后台启用的公开模型 `model` 文案，不再暴露本地直连与云端渠道差异。后端以公开模型作为计费、赠送额度资格和路由入口，选中具体服务商路由后将请求体中的 `model` 改写为 `upstreamModel`，当前所有 `upstreamModel` 默认为 `gpt-image-2`。额度拆为通用额度 `credits` 与赠送额度 `giftCredits`，只有公开模型启用 `giftEligible` 时才优先扣赠送额度。

**Tech Stack:** Go、Gin、GORM、Next.js App Router、React、TypeScript、Ant Design、Zustand、localforage。

---

## 设计边界

- `label` 不单独保存，前台展示文案与公开模型 `model` 完全一致。
- 当前图片规格使用公开模型名表达：`gpt-image-2-0.5k`、`gpt-image-2-1k`、`gpt-image-2-2k`、`gpt-image-2-4k`。
- `gpt-image-2-0.5k` 是系统可选规格，不做硬编码；后台禁用公开模型或禁用全部路由后前台不展示，后端拒绝调用。
- `gpt-image-2-1k/2k/4k` 由中转服务商路由提供，`4k` 没有启用路由时不展示。
- 服务商路由可单独配置价格、权重和实际发送给上游的模型名。
- 当前所有图片规格的 `upstreamModel` 默认使用 `gpt-image-2`，字段保留用于未来接入其它模型或服务商特殊命名。
- 后台计费以选中的路由 `credits` 为准；公开模型 `defaultCredits` 用作后台展示和无路由价格时的兜底。
- 当前项目未上线，不编写旧字段兼容迁移；数据库由 GORM `AutoMigrate` 补充新增列。

## 数据结构目标

### 公开模型配置

位于 `settings.public.modelChannel.models`：

```json
[
  {
    "model": "gpt-image-2-0.5k",
    "capability": "image",
    "enabled": true,
    "giftEligible": true,
    "defaultCredits": 1
  },
  {
    "model": "gpt-image-2-1k",
    "capability": "image",
    "enabled": true,
    "giftEligible": false,
    "defaultCredits": 8
  }
]
```

### 私有渠道路由配置

位于 `settings.private.channels[].routes`：

```json
[
  {
    "model": "gpt-image-2-1k",
    "upstreamModel": "gpt-image-2",
    "credits": 8,
    "weight": 10,
    "enabled": true
  },
  {
    "model": "gpt-image-2-2k",
    "upstreamModel": "gpt-image-2",
    "credits": 18,
    "weight": 6,
    "enabled": true
  }
]
```

### 用户额度

`users` 表新增 `gift_credits`：

```text
credits      通用额度，所有模型可用
giftCredits  赠送额度，仅 giftEligible=true 的公开模型可用
```

新用户注册后：

```text
credits = 0
giftCredits = 100
```

---

### Task 1: 后端设置模型结构

**Files:**
- Modify: `model/setting.go`
- Modify: `service/settings.go`
- Modify: `docs/content/docs/backend/system-settings.mdx`
- Modify: `docs/content/docs/backend/backend-database.mdx`

- [ ] **Step 1: 在 `model/setting.go` 增加公开模型和渠道路由结构**

将 `ModelChannel` 改为路由驱动结构，权重只放在具体 `routes` 上：

```go
type PublicModelSpec struct {
    Model          string  `json:"model"`
    Capability     string  `json:"capability"`
    Enabled        bool    `json:"enabled"`
    GiftEligible   bool    `json:"giftEligible"`
    DefaultCredits float64 `json:"defaultCredits"`
}

type ModelChannelRoute struct {
    Model         string  `json:"model"`
    UpstreamModel string  `json:"upstreamModel"`
    Credits       float64 `json:"credits"`
    Weight        int     `json:"weight"`
    Enabled       bool    `json:"enabled"`
}

type ModelChannel struct {
    Protocol string              `json:"protocol"`
    Name     string              `json:"name"`
    BaseURL  string              `json:"baseUrl"`
    APIKey   string              `json:"apiKey"`
    Routes   []ModelChannelRoute `json:"routes"`
    Enabled  bool                `json:"enabled"`
    Remark   string              `json:"remark"`
}

type PublicModelChannelSetting struct {
    Models            []PublicModelSpec `json:"models"`
    DefaultModel      string            `json:"defaultModel"`
    DefaultImageModel string            `json:"defaultImageModel"`
    DefaultVideoModel string            `json:"defaultVideoModel"`
    DefaultTextModel  string            `json:"defaultTextModel"`
    SystemPrompt      string            `json:"systemPrompt"`
    AllowCustomChannel *bool            `json:"allowCustomChannel"`
}
```

- [ ] **Step 2: 在 `service/settings.go` 规范化公开模型**

增加规范化函数，确保模型名非空、能力默认 `image`、价格不小于 0：

```go
func normalizePublicModelSpec(item model.PublicModelSpec) model.PublicModelSpec {
    item.Model = strings.TrimSpace(item.Model)
    item.Capability = strings.TrimSpace(item.Capability)
    if item.Capability == "" {
        item.Capability = "image"
    }
    if item.DefaultCredits < 0 {
        item.DefaultCredits = 0
    }
    return item
}
```

在 `normalizePublicSetting` 中替换原有 `availableModels/modelCosts` 逻辑：

```go
models := make([]model.PublicModelSpec, 0, len(setting.ModelChannel.Models))
for _, item := range setting.ModelChannel.Models {
    normalized := normalizePublicModelSpec(item)
    if normalized.Model == "" {
        continue
    }
    models = append(models, normalized)
}
setting.ModelChannel.Models = models
```

- [ ] **Step 3: 在 `service/settings.go` 规范化渠道路由**

新增路由规范化函数：

```go
func normalizeModelChannelRoute(route model.ModelChannelRoute) model.ModelChannelRoute {
    route.Model = strings.TrimSpace(route.Model)
    route.UpstreamModel = strings.TrimSpace(route.UpstreamModel)
    if route.UpstreamModel == "" {
        route.UpstreamModel = "gpt-image-2"
    }
    if route.Credits < 0 {
        route.Credits = 0
    }
    if route.Weight <= 0 {
        route.Weight = 1
    }
    return route
}
```

在 `normalizeModelChannel` 中规范化 `Routes`：

```go
routes := make([]model.ModelChannelRoute, 0, len(channel.Routes))
for _, route := range channel.Routes {
    normalized := normalizeModelChannelRoute(route)
    if normalized.Model == "" {
        continue
    }
    routes = append(routes, normalized)
}
channel.Routes = routes
```

- [ ] **Step 4: 更新系统配置文档**

在 `docs/content/docs/backend/system-settings.mdx` 和 `docs/content/docs/backend/backend-database.mdx` 中说明：

```markdown
| `models` | object[] | 公开模型规格列表，前台只展示启用且存在可用路由的模型 |
| `models[].model` | string | 公开模型名，同时也是前台展示文案 |
| `models[].capability` | string | 能力类型：`image`、`video`、`text`、`audio` |
| `models[].enabled` | boolean | 是否启用该公开模型 |
| `models[].giftEligible` | boolean | 是否允许优先消耗系统赠送额度 |
| `models[].defaultCredits` | number | 默认算力点价格，路由未配置价格时使用 |
| `private.channels[].routes` | object[] | 渠道支持的公开模型路由 |
| `routes[].model` | string | 对应公开模型名 |
| `routes[].upstreamModel` | string | 实际发送给上游接口的模型名，当前默认 `gpt-image-2` |
| `routes[].credits` | number | 该服务商路由的单次算力点价格 |
| `routes[].weight` | number | 该路由在同一公开模型下的权重 |
| `routes[].enabled` | boolean | 是否启用该路由 |
```

---

### Task 2: 公开模型可用性与路由选择服务

**Files:**
- Modify: `service/settings.go`
- Modify: `handler/ai.go`
- Test: `service/settings_test.go`

- [ ] **Step 1: 新增路由解析结果类型**

在 `service/settings.go` 增加：

```go
type SelectedModelRoute struct {
    PublicModel   model.PublicModelSpec
    Channel       model.ModelChannel
    Route         model.ModelChannelRoute
    PublicModelName string
    UpstreamModel string
    Credits       float64
}
```

- [ ] **Step 2: 新增公开模型查找函数**

```go
func publicModelForName(settings model.Settings, modelName string) (model.PublicModelSpec, bool) {
    for _, item := range normalizePublicSetting(settings.Public).ModelChannel.Models {
        if item.Enabled && strings.TrimSpace(item.Model) == strings.TrimSpace(modelName) {
            return item, true
        }
    }
    return model.PublicModelSpec{}, false
}
```

- [ ] **Step 3: 新增按公开模型选择渠道路由函数**

```go
func SelectModelRoute(modelName string) (SelectedModelRoute, error) {
    settings, err := repository.GetSettings()
    if err != nil {
        return SelectedModelRoute{}, err
    }
    settings = normalizeSettings(settings)
    publicModel, ok := publicModelForName(settings, modelName)
    if !ok {
        return SelectedModelRoute{}, safeMessageError{message: "模型未启用或不存在"}
    }

    candidates := []SelectedModelRoute{}
    for _, channel := range normalizePrivateSetting(settings.Private).Channels {
        if !channel.Enabled || strings.TrimSpace(channel.BaseURL) == "" || strings.TrimSpace(channel.APIKey) == "" {
            continue
        }
        for _, route := range channel.Routes {
            if !route.Enabled || strings.TrimSpace(route.Model) != modelName {
                continue
            }
            credits := route.Credits
            if credits <= 0 {
                credits = publicModel.DefaultCredits
            }
            upstreamModel := strings.TrimSpace(route.UpstreamModel)
            if upstreamModel == "" {
                upstreamModel = "gpt-image-2"
            }
            candidates = append(candidates, SelectedModelRoute{
                PublicModel: publicModel,
                Channel: channel,
                Route: route,
                PublicModelName: modelName,
                UpstreamModel: upstreamModel,
                Credits: credits,
            })
        }
    }
    if len(candidates) == 0 {
        return SelectedModelRoute{}, safeMessageError{message: "没有可用模型渠道"}
    }

    total := 0
    for _, item := range candidates {
        total += item.Route.Weight
    }
    hit := rand.Intn(total)
    for _, item := range candidates {
        hit -= item.Route.Weight
        if hit < 0 {
            return item, nil
        }
    }
    return candidates[0], nil
}
```

- [ ] **Step 4: 新增单元测试覆盖启用、禁用、权重候选和价格兜底**

在 `service/settings_test.go` 增加测试用例：

```go
func TestSelectModelRouteUsesEnabledPublicModelAndRouteCredits(t *testing.T) {
    // 准备 settings：公开模型 gpt-image-2-1k 启用，两个渠道各有 route。
    // 调用 SelectModelRoute("gpt-image-2-1k")。
    // 断言返回 PublicModelName 为 gpt-image-2-1k，UpstreamModel 为 gpt-image-2，Credits 来自 route.Credits。
}

func TestSelectModelRouteRejectsDisabledPublicModel(t *testing.T) {
    // 准备 settings：公开模型 gpt-image-2-0.5k disabled。
    // 调用 SelectModelRoute("gpt-image-2-0.5k")。
    // 断言返回错误信息为“模型未启用或不存在”。
}
```

Run: `go test ./service`

Expected: service 包测试通过。

---

### Task 3: AI 代理改写上游模型

**Files:**
- Modify: `handler/ai.go`
- Test: `handler/ai_test.go`

- [ ] **Step 1: 将 `proxyAIRequest` 从 `ModelCost + SelectModelChannel` 切换为 `SelectModelRoute`**

替换当前流程：

```go
credits, err := service.ModelCost(modelName)
channel, err := service.SelectModelChannel(modelName)
```

改为：

```go
selected, err := service.SelectModelRoute(modelName)
if err != nil {
    FailError(w, err)
    return
}
credits := selected.Credits * readAIRequestCount(body, contentType)
```

- [ ] **Step 2: 增加 JSON 请求体模型改写**

```go
func rewriteAIRequestModel(body []byte, contentType string, upstreamModel string) ([]byte, string, error) {
    if strings.HasPrefix(contentType, "multipart/form-data") {
        return rewriteMultipartModel(body, contentType, upstreamModel)
    }
    var payload map[string]any
    if err := json.Unmarshal(body, &payload); err != nil {
        return nil, "", err
    }
    payload["model"] = upstreamModel
    nextBody, err := json.Marshal(payload)
    return nextBody, contentType, err
}
```

- [ ] **Step 3: 增加 multipart 请求体模型改写**

```go
func rewriteMultipartModel(body []byte, contentType string, upstreamModel string) ([]byte, string, error) {
    _, params, err := mime.ParseMediaType(contentType)
    if err != nil {
        return nil, "", err
    }
    reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
    form, err := reader.ReadForm(64 << 20)
    if err != nil {
        return nil, "", err
    }
    defer form.RemoveAll()

    var buffer bytes.Buffer
    writer := multipart.NewWriter(&buffer)
    for key, values := range form.Value {
        for _, value := range values {
            if key == "model" {
                value = upstreamModel
            }
            _ = writer.WriteField(key, value)
        }
    }
    if len(form.Value["model"]) == 0 {
        _ = writer.WriteField("model", upstreamModel)
    }
    for key, files := range form.File {
        for _, fileHeader := range files {
            source, err := fileHeader.Open()
            if err != nil {
                return nil, "", err
            }
            target, err := writer.CreateFormFile(key, fileHeader.Filename)
            if err != nil {
                _ = source.Close()
                return nil, "", err
            }
            _, copyErr := io.Copy(target, source)
            _ = source.Close()
            if copyErr != nil {
                return nil, "", copyErr
            }
        }
    }
    if err := writer.Close(); err != nil {
        return nil, "", err
    }
    return buffer.Bytes(), writer.FormDataContentType(), nil
}
```

- [ ] **Step 4: 在代理请求中使用改写后的 body 和 contentType**

```go
body, contentType, err = rewriteAIRequestModel(body, contentType, selected.UpstreamModel)
if err != nil {
    Fail(w, "AI 接口请求失败")
    return
}
request, err := http.NewRequest(http.MethodPost, service.BuildModelChannelURL(selected.Channel, path), bytes.NewReader(body))
```

- [ ] **Step 5: 记录公开模型和上游模型**

扣费流水传入公开模型、上游模型、渠道名和路径。`handler/ai.go` 调用应传递 `selected`：

```go
if err := service.ConsumeUserCredits(user.ID, selected, credits, path); err != nil {
    FailError(w, err)
    return
}
```

- [ ] **Step 6: 增加测试**

在 `handler/ai_test.go` 增加：

```go
func TestRewriteAIRequestModelJSON(t *testing.T) {
    body := []byte(`{"model":"gpt-image-2-2k","prompt":"画一只猫"}`)
    next, contentType, err := rewriteAIRequestModel(body, "application/json", "gpt-image-2")
    if err != nil {
        t.Fatal(err)
    }
    if contentType != "application/json" {
        t.Fatalf("contentType = %s", contentType)
    }
    if !bytes.Contains(next, []byte(`"model":"gpt-image-2"`)) {
        t.Fatalf("body not rewritten: %s", string(next))
    }
}
```

Run: `go test ./handler`

Expected: handler 包测试通过。

---

### Task 4: 通用额度与赠送额度分账

**Files:**
- Modify: `model/user.go`
- Modify: `repository/user.go`
- Modify: `service/auth.go`
- Modify: `handler/ai.go`
- Modify: `docs/content/docs/backend/backend-database.mdx`

- [ ] **Step 1: 用户模型增加赠送额度**

在 `model.User` 和 `model.AuthUser` 增加：

```go
GiftCredits float64 `json:"giftCredits"`
```

在 `PublicUser` 中返回：

```go
GiftCredits: user.GiftCredits,
```

- [ ] **Step 2: 新用户注册写入赠送额度**

在账号密码注册和 Linux.do 新用户注册处改为：

```go
Credits:     0,
GiftCredits: 100,
```

注册赠送流水改为：

```go
Balance:   user.Credits,
Extra:     `{"giftCredits":100,"giftBalance":100}`,
Remark:    "新用户注册赠送，仅限支持赠送额度的模型",
```

- [ ] **Step 3: 定义扣费结果**

在 `service/auth.go` 或独立小文件中增加：

```go
type CreditDebit struct {
    Credits     float64
    GiftCredits float64
    Total       float64
}
```

- [ ] **Step 4: 仓储层增加原子扣费**

在 `repository/user.go` 增加事务函数：

```go
func ConsumeUserCreditBuckets(id string, debit model.CreditDebit, now string) (model.User, bool, error) {
    db, err := DB()
    if err != nil {
        return model.User{}, false, err
    }
    var user model.User
    err = db.Transaction(func(tx *gorm.DB) error {
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&user).Error; err != nil {
            return err
        }
        if user.Credits < debit.Credits || user.GiftCredits < debit.GiftCredits {
            return gorm.ErrRecordNotFound
        }
        return tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
            "credits":      gorm.Expr("credits - ?", debit.Credits),
            "gift_credits": gorm.Expr("gift_credits - ?", debit.GiftCredits),
            "updated_at":   now,
        }).Error
    })
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return model.User{}, false, nil
    }
    if err != nil {
        return model.User{}, false, err
    }
    user, ok, err := GetUserByID(id)
    return user, ok, err
}
```

`CreditDebit` 类型放在 `model` 包，避免 `repository` 与 `service` 形成循环引用。

- [ ] **Step 5: 仓储层增加原路退款**

```go
func RefundUserCreditBuckets(id string, debit model.CreditDebit, now string) (model.User, bool, error) {
    db, err := DB()
    if err != nil {
        return model.User{}, false, err
    }
    tx := db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
        "credits":      gorm.Expr("credits + ?", debit.Credits),
        "gift_credits": gorm.Expr("gift_credits + ?", debit.GiftCredits),
        "updated_at":   now,
    })
    if tx.Error != nil {
        return model.User{}, false, tx.Error
    }
    user, ok, err := GetUserByID(id)
    return user, ok && tx.RowsAffected > 0, err
}
```

- [ ] **Step 6: 服务层计算扣费桶**

```go
func buildCreditDebit(user model.User, selected SelectedModelRoute, total float64) (model.CreditDebit, bool) {
    debit := model.CreditDebit{Total: total}
    if selected.PublicModel.GiftEligible {
        debit.GiftCredits = math.Min(user.GiftCredits, total)
    }
    debit.Credits = total - debit.GiftCredits
    return debit, user.Credits >= debit.Credits && user.GiftCredits >= debit.GiftCredits
}
```

- [ ] **Step 7: 服务层扣费和退款记录分账信息**

将 `ConsumeUserCredits` 签名改为：

```go
func ConsumeUserCredits(userID string, selected SelectedModelRoute, total float64, path string) (model.CreditDebit, error)
```

流水 `Extra` 写入：

```go
extra, _ := json.Marshal(map[string]any{
    "publicModel": selected.PublicModelName,
    "upstreamModel": selected.UpstreamModel,
    "channel": selected.Channel.Name,
    "credits": debit.Credits,
    "giftCredits": debit.GiftCredits,
    "path": path,
})
```

退款函数签名改为：

```go
func RefundUserCredits(userID string, selected SelectedModelRoute, debit model.CreditDebit, path string) error
```

失败退款必须使用实际扣除的 `debit`，不可重新计算。

- [ ] **Step 8: 更新数据库文档**

在 `users` 表字段说明增加：

```markdown
| `gift_credits` | number | 系统赠送额度余额，仅可用于 `giftEligible=true` 的公开模型 |
```

在 `credit_logs.extra` 说明增加：

```markdown
AI 扣费和退款流水会记录 `publicModel`、`upstreamModel`、`channel`、`credits`、`giftCredits` 和 `path`，用于区分通用额度与赠送额度。
```

---

### Task 5: 前端配置与模型选择

**Files:**
- Modify: `web/src/services/api/admin.ts`
- Modify: `web/src/stores/use-config-store.ts`
- Modify: `web/src/components/layout/app-config-modal.tsx`
- Modify: `web/src/components/model-picker.tsx`
- Modify: `web/src/constant/credits.tsx`
- Modify: `web/src/components/layout/user-status-actions.tsx`

- [ ] **Step 1: 更新前端设置类型**

在 `web/src/services/api/admin.ts` 增加：

```ts
export type AdminPublicModelSpec = {
    model: string;
    capability: "image" | "video" | "text" | "audio";
    enabled: boolean;
    giftEligible: boolean;
    defaultCredits: number;
};

export type AdminModelChannelRoute = {
    model: string;
    upstreamModel: string;
    credits: number;
    weight: number;
    enabled: boolean;
};
```

更新：

```ts
export type AdminModelChannel = {
    protocol: "openai";
    name: string;
    baseUrl: string;
    apiKey: string;
    routes: AdminModelChannelRoute[];
    enabled: boolean;
    remark: string;
};

export type AdminPublicModelChannelSettings = {
    models: AdminPublicModelSpec[];
    defaultModel: string;
    defaultImageModel: string;
    defaultVideoModel: string;
    defaultTextModel: string;
    systemPrompt: string;
    allowCustomChannel: boolean;
};
```

- [ ] **Step 2: `use-config-store` 从公开模型列表派生各能力模型**

新增工具函数：

```ts
function enabledPublicModelsByCapability(models: AdminPublicModelSpec[], capability: ModelCapability) {
    return models.filter((item) => item.enabled && item.capability === capability).map((item) => item.model);
}
```

在 `resolveEffectiveConfig` 中使用：

```ts
const imageModels = enabledPublicModelsByCapability(modelChannel.models, "image");
const videoModels = enabledPublicModelsByCapability(modelChannel.models, "video");
const textModels = enabledPublicModelsByCapability(modelChannel.models, "text");
const audioModels = enabledPublicModelsByCapability(modelChannel.models, "audio");
```

- [ ] **Step 3: 配置弹窗隐藏本地直连入口**

在 `AppConfigModal` 中删除或不渲染“渠道模式”分段控件、本地 `Base URL/API Key`、本地模型列表和本地系统提示词。保留说明块：

```tsx
<div className="mb-5 rounded-lg border border-stone-200 p-3 text-sm text-stone-500 dark:border-stone-800">
    <div className="font-medium text-stone-900 dark:text-stone-100">系统模型渠道</div>
    <div className="mt-1">所有请求由后台模型渠道转发，当前可用 {modelChannel?.models.filter((item) => item.enabled).length || 0} 个模型。</div>
</div>
```

保存配置时强制：

```ts
if (config.channelMode !== "remote") updateConfig("channelMode", "remote");
```

- [ ] **Step 4: 模型选择器直接显示 model 原文**

`ModelPicker` 不做 label 转换，`options` 使用：

```ts
const options = selectableModelsByCapability(config, capability).map((model) => ({
    label: model,
    value: model,
}));
```

- [ ] **Step 5: 额度展示支持赠送额度**

`use-user-store` 的用户类型增加：

```ts
giftCredits: number;
```

`UserStatusActions` 中显示：

```tsx
<span>{credits.toLocaleString()}</span>
<span className="text-xs opacity-60">赠送 {giftCredits.toLocaleString()}</span>
```

- [ ] **Step 6: 前端请求费用展示使用公开模型默认价**

`web/src/constant/credits.tsx` 调整为接收公开模型：

```ts
export type ModelCreditCost = {
    model: string;
    credits: number;
    giftEligible?: boolean;
};
```

页面中显示的费用使用 `defaultCredits` 派生，实际扣费以后端选中路由为准。文案使用“预计”：

```tsx
预计消耗 {credits.toLocaleString()}
```

---

### Task 6: 后台设置页面升级

**Files:**
- Modify: `web/src/app/(admin)/admin/settings/page.tsx`

- [ ] **Step 1: 默认空配置改为公开模型列表和渠道路由**

```ts
const emptySettings: AdminSettings = {
    public: {
        modelChannel: {
            models: [],
            defaultModel: "",
            defaultImageModel: "",
            defaultVideoModel: "",
            defaultTextModel: "",
            systemPrompt: "",
            allowCustomChannel: false,
        },
        auth: { allowRegister: true, linuxDo: { enabled: false } },
    },
    private: { channels: [], promptSync: { enabled: true, cron: "*/5 * * * *" }, auth: { linuxDo: { clientId: "", clientSecret: "" } } },
};

const emptyChannel: AdminModelChannel = {
    protocol: "openai",
    name: "",
    baseUrl: "",
    apiKey: "",
    routes: [],
    enabled: true,
    remark: "",
};
```

- [ ] **Step 2: 公开配置页增加公开模型表格**

表格列：

```text
模型 model
能力 capability
启用 enabled
允许赠送额度 giftEligible
默认价格 defaultCredits
操作
```

增加快速插入按钮，写入以下四个模型：

```ts
const gptImage2Specs: AdminPublicModelSpec[] = [
    { model: "gpt-image-2-0.5k", capability: "image", enabled: true, giftEligible: true, defaultCredits: 1 },
    { model: "gpt-image-2-1k", capability: "image", enabled: true, giftEligible: false, defaultCredits: 8 },
    { model: "gpt-image-2-2k", capability: "image", enabled: true, giftEligible: false, defaultCredits: 18 },
    { model: "gpt-image-2-4k", capability: "image", enabled: false, giftEligible: false, defaultCredits: 36 },
];
```

- [ ] **Step 3: 渠道抽屉增加路由表格**

替换原 `models` 多选为 `routes` 表格。每条路由字段：

```text
公开模型 model，下拉来源为 public.modelChannel.models
上游模型 upstreamModel，默认 gpt-image-2
价格 credits
权重 weight
启用 enabled
```

新增路由按钮默认值：

```ts
{
    model: publicModels[0]?.model || "",
    upstreamModel: "gpt-image-2",
    credits: publicModels[0]?.defaultCredits || 0,
    weight: 1,
    enabled: true,
}
```

- [ ] **Step 4: 保存设置时从公开模型和路由计算默认模型**

`collectSettings` 中选择启用且能力匹配的第一个模型作为默认兜底：

```ts
const imageModels = values.public.modelChannel.models.filter((item) => item.enabled && item.capability === "image").map((item) => item.model);
values.public.modelChannel.defaultImageModel = imageModels.includes(values.public.modelChannel.defaultImageModel) ? values.public.modelChannel.defaultImageModel : imageModels[0] || "";
```

其它能力按相同规则处理。

- [ ] **Step 5: 关闭未配置路由的公开模型**

保存时根据启用渠道路由计算有服务能力的模型集合：

```ts
const routableModels = new Set(values.private.channels.filter((channel) => channel.enabled).flatMap((channel) => channel.routes.filter((route) => route.enabled).map((route) => route.model)));
values.public.modelChannel.models = values.public.modelChannel.models.map((item) => ({
    ...item,
    enabled: item.enabled && routableModels.has(item.model),
}));
```

这保证 `gpt-image-2-4k` 没有服务商 route 时不会展示。

---

### Task 7: 图片请求规格参数

**Files:**
- Modify: `web/src/services/api/image.ts`
- Modify: `web/src/components/image-settings-panel.tsx`
- Modify: `web/src/app/(user)/canvas/components/canvas-image-settings-popover.tsx`

- [ ] **Step 1: 规格模型与尺寸规则保持解耦**

前端继续发送用户选择的公开模型，例如 `gpt-image-2-2k`。后端改写 `model` 为 `gpt-image-2`，但不改写 `size` 和 `quality`。

- [ ] **Step 2: 保留现有图片尺寸控件**

`image-settings-panel.tsx` 中的 1k/2k/4k 尺寸选项继续作为请求 `size` 参数，不承担渠道路由职责。

- [ ] **Step 3: 如需自动匹配规格尺寸，在前端增加纯函数**

```ts
export function defaultImageSizeForModel(model: string) {
    if (model.endsWith("-0.5k")) return "1024x1024";
    if (model.endsWith("-1k")) return "1024x1024";
    if (model.endsWith("-2k")) return "2048x2048";
    if (model.endsWith("-4k")) return "3840x2160";
    return "auto";
}
```

该函数只用于用户切换模型时填充默认尺寸，不参与后端计费和渠道选择。

---

### Task 8: 文档与进度留痕

**Files:**
- Modify: `docs/content/docs/backend/system-settings.mdx`
- Modify: `docs/content/docs/backend/backend-database.mdx`
- Modify: `docs/content/docs/progress/pending-test.mdx`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: 更新系统配置文档**

记录公开模型、私有渠道路由、上游模型改写和权重选择规则。

- [ ] **Step 2: 更新数据库文档**

记录 `users.gift_credits` 和 `credit_logs.extra` 的分账字段。

- [ ] **Step 3: 更新待测试文档**

在 `pending-test.mdx` 增加：

```markdown
- 模型配置升级为公开模型规格和私有渠道路由：前台只展示后台启用且有可用路由的模型，后台可配置不同服务商的路由权重、上游模型名和单独价格。
- 用户额度拆分为通用额度和赠送额度：系统赠送额度仅可用于启用 `giftEligible` 的公开模型，模型调用失败时按实际扣除来源原路退回。
```

- [ ] **Step 4: 更新 CHANGELOG**

在 `Unreleased` 中归纳：

```markdown
- 调整模型渠道与额度体系，支持公开模型规格、私有渠道路由、按服务商权重分配和赠送额度限制。
```

---

## Verification

按项目约束，实际开发完成后不强制执行构建。建议由用户或开发者自行选择执行以下检查：

```bash
go test ./service ./handler ./repository
```

预期结果：

```text
ok github.com/basketikun/infinite-canvas/service
ok github.com/basketikun/infinite-canvas/handler
ok github.com/basketikun/infinite-canvas/repository
```

手动验证路径：

1. 后台新增公开模型 `gpt-image-2-0.5k`，启用 `giftEligible`，配置系统自营 route，前台可以选择该模型。
2. 后台禁用 `gpt-image-2-0.5k` 或禁用全部对应 route，前台不再展示该模型。
3. 用户只有 `giftCredits` 时，可以调用 `gpt-image-2-0.5k`，不能调用 `gpt-image-2-1k/2k/4k`。
4. 用户有通用额度时，可以调用 `gpt-image-2-1k/2k/4k`。
5. 多个渠道都配置 `gpt-image-2-2k` route 时，后端按各自 `routes[].weight` 分配。
6. 上游收到的请求模型名为 `gpt-image-2`，后台流水记录公开模型为 `gpt-image-2-2k`。
7. 上游失败时，通用额度和赠送额度都按实际扣除数量退回。

## Open Decisions

- 当前计划让前端显示“预计消耗”，实际扣费按后端选中的 route 价格执行；如果需要用户点击前展示精确价格，后端需要公开每个公开模型的价格范围或固定统一公开价格。
- 当前 `upstreamModel` 默认 `gpt-image-2`；未来接其它图片模型时，可以在 route 中单独配置，无需改变公开模型展示规则。

## Next Skill

`$superpower-subagents` 或 `$superpower-executing-plans`
