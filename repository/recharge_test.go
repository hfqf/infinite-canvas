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

	for _, item := range []struct {
		amount      int
		credits     int
		memberType  model.MemberType
		memberLevel model.MemberLevel
		productName string
	}{
		{amount: 59, credits: 590, memberType: model.MemberTypeMonthly, memberLevel: model.MemberLevelBasic, productName: "好图秀AI算力充值-月度-基础版"},
		{amount: 99, credits: 1100, memberType: model.MemberTypeMonthly, memberLevel: model.MemberLevelAdvanced, productName: "好图秀AI算力充值-月度-高级版"},
		{amount: 199, credits: 2488, memberType: model.MemberTypeMonthly, memberLevel: model.MemberLevelPremium, productName: "好图秀AI算力充值-月度-尊享版"},
		{amount: 499, credits: 5000, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelStandard, productName: "好图秀AI算力充值-年度-普通版"},
		{amount: 699, credits: 6996, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelBasic, productName: "好图秀AI算力充值-年度-基础版"},
		{amount: 999, credits: 10020, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelAdvanced, productName: "好图秀AI算力充值-年度-高级版"},
		{amount: 1999, credits: 21044, memberType: model.MemberTypeAnnual, memberLevel: model.MemberLevelPremium, productName: "好图秀AI算力充值-年度-尊享版"},
	} {
		order, err := model.NewRechargeOrder("user_1", item.amount, "now")
		if err != nil {
			t.Fatal(err)
		}
		if order.AmountFen != item.amount*100 || order.Credits != item.credits || order.MemberType != item.memberType || order.MemberLevel != item.memberLevel || order.ProductName != item.productName {
			t.Fatalf("amount %d order=%#v, want amountFen=%d credits=%d memberType=%s memberLevel=%s productName=%s", item.amount, order, item.amount*100, item.credits, item.memberType, item.memberLevel, item.productName)
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
