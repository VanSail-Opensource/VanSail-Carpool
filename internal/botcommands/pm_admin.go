package botcommands

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

// PMAdminCopy contains localized replies for commands used in administrator PM topics.
type PMAdminCopy struct {
	Title, Status, Usage, Rejected, Added, Deducted, RefundUnavailable, RefundQueued string
	Amount, Balance, Receipt                                                         string
}

// PMAdminTexts returns the copy set for topic-scoped administrator commands.
func PMAdminTexts(language Language) PMAdminCopy {
	if language == Chinese {
		return PMAdminCopy{
			Title: "用户账户命令", Status: "状态",
			Usage:    "请在用户私信话题中使用 /refund、/addtxb <数值> 或 /deducttxb <数值>。",
			Rejected: "命令未能完成。请检查金额、当前套餐和用户余额。",
			Added:    "余额已增加。", Deducted: "余额已扣减。",
			RefundUnavailable: "无法根据已验证的流量使用量计算当前套餐退款。",
			RefundQueued:      "当前套餐退款已记录，上游同步已加入队列。",
			Amount:            "金额", Balance: "余额", Receipt: "操作编号",
		}
	}
	return PMAdminCopy{
		Title: "Member account command", Status: "Status",
		Usage:    "Use /refund, /addtxb <value>, or /deducttxb <value> in a member's PM topic.",
		Rejected: "The command could not be completed. Check the amount, active combo, and available balance.",
		Added:    "Balance increased.", Deducted: "Balance decreased.",
		RefundUnavailable: "The current combo refund could not be calculated from verified traffic usage.",
		RefundQueued:      "The current combo refund was recorded and its provider update was queued.",
		Amount:            "Amount", Balance: "Balance", Receipt: "Operation",
	}
}

// PMAdminDescriptions returns commands that should only be registered in the PM forum.
func PMAdminDescriptions(language Language) []Description {
	if language == Chinese {
		return []Description{{Refund, "退还当前套餐费用"}, {AddTXB, "增加用户 TXB 余额"}, {DeductTXB, "扣减用户 TXB 余额"}}
	}
	return []Description{{Refund, "Refund this member's current combo"}, {AddTXB, "Add TXB to this member's balance"}, {DeductTXB, "Deduct TXB from this member's balance"}}
}

// FormatPMAdminUsage renders localized PM command usage.
func FormatPMAdminUsage(copy PMAdminCopy) string {
	return formatCard(copy.Title, cardField{copy.Status, copy.Usage})
}

// FormatPMAdminRejected renders a safe command failure.
func FormatPMAdminRejected(copy PMAdminCopy) string {
	return formatCard(copy.Title, cardField{copy.Status, copy.Rejected})
}

// FormatPMAdminBalance renders an audited balance adjustment result.
func FormatPMAdminBalance(copy PMAdminCopy, added bool, amount, balance model.Money) string {
	status := copy.Deducted
	if added {
		status = copy.Added
	}
	return formatCard(copy.Title, cardField{copy.Status, status}, cardField{copy.Amount, amount.Display}, cardField{copy.Balance, balance.Display})
}

// FormatPMRefundUnavailable explains why no traffic-based refund was issued.
func FormatPMRefundUnavailable(copy PMAdminCopy) string {
	return formatCard(copy.Title, cardField{copy.Status, copy.RefundUnavailable})
}

// FormatPMRefundQueued confirms a durable entitlement-refund receipt.
func FormatPMRefundQueued(copy PMAdminCopy, amount model.Money, operationID string) string {
	return formatCard(copy.Title, cardField{copy.Status, copy.RefundQueued}, cardField{copy.Amount, amount.Display}, cardField{copy.Receipt, operationID})
}
