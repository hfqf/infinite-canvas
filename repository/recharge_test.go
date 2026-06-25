package repository

import (
	"testing"

	"github.com/basketikun/infinite-canvas/model"
)

func TestRechargeAmountValidation(t *testing.T) {
	for _, amount := range []int{0, 1, 58, 60, 1000, 1200} {
		if _, err := model.NewRechargeOrder("user_1", amount, "now"); err == nil {
			t.Fatalf("amount %d accepted, want validation error", amount)
		}
	}
	for _, amountFen := range []int{0, 1, 5, 9, 11, 58, 60, 1000, 1200} {
		if _, err := model.NewRechargeOrderByAmountFen("user_1", amountFen, "now"); err == nil {
			t.Fatalf("amountFen %d accepted, want validation error", amountFen)
		}
	}

	for _, item := range []struct {
		amountYuan  int
		amountFen   int
		credits     int
		memberType  model.MemberType
		memberLevel model.MemberLevel
		productName string
	}{
		{amountYuan: 59, amountFen: 5900, credits: 590, memberType: model.MemberTypeMonthly, memberLevel: model.MemberLevelBasic, productName: "好图秀AI积分充值-月度-基础版"},
		{amountYuan: 99, amountFen: 9900, credits: 1100, memberType: model.MemberTypeMonthly, memberLevel: model.MemberLevelAdvanced, productName: "好图秀AI积分充值-月度-高级版"},
		{amountYuan: 199, amountFen: 19900, credits: 2488, memberType: model.MemberTypeMonthly, memberLevel: model.MemberLevelPremium, productName: "好图秀AI积分充值-月度-尊享版"},
		{amountYuan: 499, amountFen: 49900, credits: 5000, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelStandard, productName: "好图秀AI积分充值-年度-普通版"},
		{amountYuan: 699, amountFen: 69900, credits: 6996, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelBasic, productName: "好图秀AI积分充值-年度-基础版"},
		{amountYuan: 999, amountFen: 99900, credits: 10020, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelAdvanced, productName: "好图秀AI积分充值-年度-高级版"},
		{amountYuan: 1999, amountFen: 199900, credits: 21044, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelPremium, productName: "好图秀AI积分充值-年度-尊享版"},
		{amountYuan: 0, amountFen: 10, credits: 1, memberType: model.MemberTypeTest, memberLevel: model.MemberLevelTest, productName: "好图秀AI积分充值-测试-0.10元"},
		{amountYuan: 0, amountFen: 50, credits: 5, memberType: model.MemberTypeTest, memberLevel: model.MemberLevelTest, productName: "好图秀AI积分充值-测试-0.50元"},
	} {
		order, err := model.NewRechargeOrderByAmountFen("user_1", item.amountFen, "now")
		if err != nil {
			t.Fatal(err)
		}
		if order.AmountYuan != item.amountYuan || order.AmountFen != item.amountFen || order.Credits != item.credits || order.MemberType != item.memberType || order.MemberLevel != item.memberLevel || order.ProductName != item.productName {
			t.Fatalf("amountFen %d order=%#v, want amountYuan=%d amountFen=%d credits=%d memberType=%s memberLevel=%s productName=%s", item.amountFen, order, item.amountYuan, item.amountFen, item.credits, item.memberType, item.memberLevel, item.productName)
		}
	}
}

func TestCompleteRechargeOrderPaidIsIdempotent(t *testing.T) {
	resetDBForTest(t)
	user, err := SaveUser(model.User{ID: "user_recharge_001", Username: "recharge-user", Role: model.UserRoleUser, Status: model.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	order, err := model.NewRechargeOrder(user.ID, 999, "created")
	if err != nil {
		t.Fatal(err)
	}
	order.ID = "recharge_order_001"
	order.OutTradeNo = "recharge_recharge_order_001"
	if order, err = SaveRechargeOrder(order); err != nil {
		t.Fatal(err)
	}

	paid, err := CompleteRechargeOrderPaid(order.OutTradeNo, order.AmountFen, "wx_tx_001", "paid")
	if err != nil {
		t.Fatal(err)
	}
	if !paid {
		t.Fatal("first completion returned paid=false")
	}
	if paid, err = CompleteRechargeOrderPaid(order.OutTradeNo, order.AmountFen, "wx_tx_001", "paid-again"); err != nil {
		t.Fatal(err)
	} else if paid {
		t.Fatal("second completion returned paid=true, want idempotent false")
	}

	user, ok, err := GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("load user ok=%v err=%v", ok, err)
	}
	if user.Credits != 10020 {
		t.Fatalf("credits=%d, want 10020", user.Credits)
	}
	if user.MemberType != model.MemberTypeAnnual || user.MemberLevel != model.MemberLevelAdvanced || user.LastRechargeAmountYuan != 999 || user.LastRechargedAt != "paid" {
		t.Fatalf("user recharge fields=%#v, want annual/advanced/999/paid", user)
	}
	logs, total, err := ListCreditLogs(model.Query{Keyword: order.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(logs) != 1 || logs[0].Amount != 10020 {
		t.Fatalf("logs=%#v total=%d, want one +10020 log", logs, total)
	}
}
