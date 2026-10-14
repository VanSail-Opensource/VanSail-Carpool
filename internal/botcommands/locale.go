package botcommands

import "strings"

// Language is a supported bot reply locale.
type Language string

const (
	English Language = "en"
	Chinese Language = "zh-CN"
)

// Description is one localized Bot API command description.
type Description struct {
	Name        Name
	Description string
}

// Copy owns every user-visible bot string.
type Copy struct {
	Descriptions          []Description
	Start                 string
	WelcomePrefix         string
	WelcomeSuffix         string
	MemberLabel           string
	UnknownTitle          string
	Unknown               string
	UnavailableTitle      string
	Unavailable           string
	NoSubscriptionTitle   string
	NoSubscription        string
	SubscriptionTitle     string
	ComboTitle            string
	DeductTitle           string
	StatusLabel           string
	AmountLabel           string
	BalanceLabel          string
	ComboLabel            string
	SquadsLabel           string
	TrafficLabel          string
	ResetLabel            string
	ResetDaily            string
	ResetWeekly           string
	ResetMonthly          string
	RolloverLabel         string
	RolloverWill          string
	RolloverWillNot       string
	RolloverCannot        string
	RolloverUnavailable   string
	SignInReward          string
	SignInAbove           string
	SignInBelow           string
	SignInEqual           string
	SignInNeutral         string
	SignInAlready         string
	GroupBoostRequired    string
	GroupBoostUnavailable string
	DeductUsage           string
	DeductRejected        string
	DeductSucceeded       string
}

// LanguageFor maps all Telegram Chinese variants to Simplified Chinese.
func LanguageFor(code string) Language {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(code)), "zh") {
		return Chinese
	}
	return English
}

// Text returns the immutable copy set for a language.
func Text(language Language) Copy {
	var copy Copy
	if language == Chinese {
		copy = chineseCopy()
	} else {
		copy = englishCopy()
	}
	copy.Descriptions = append(copy.Descriptions, PMAdminDescriptions(language)...)
	return copy
}

func englishCopy() Copy {
	return Copy{
		Descriptions: []Description{{Sub, "Show subscription usage"}, {Balance, "Show your TXB balance"}, {SignIn, "Claim today's check-in"}, {Start, "Open VanSail Carpool"}, {MyCombo, "Show combo and rollover details"}},
		Start:        "Press Open VanSail Carpool to use the app. For help, send a message directly here after opening the panel and completing entry verification.", WelcomePrefix: "👋 Welcome ", WelcomeSuffix: " to VanSail Carpool!", MemberLabel: "new member",
		UnknownTitle: "Command unavailable", Unknown: "This command is not available.",
		UnavailableTitle: "Temporarily unavailable", Unavailable: "The requested information is temporarily unavailable.",
		NoSubscriptionTitle: "Subscription", NoSubscription: "No active subscription.", SubscriptionTitle: "My subscription", ComboTitle: "My combo", DeductTitle: "Balance deduction",
		StatusLabel: "Status", AmountLabel: "Amount", BalanceLabel: "Balance",
		ComboLabel: "Combo", SquadsLabel: "Squads", TrafficLabel: "Traffic", ResetLabel: "Reset",
		ResetDaily: "Daily", ResetWeekly: "Weekly", ResetMonthly: "Rolling monthly", RolloverLabel: "Rollover",
		RolloverWill: "Expected", RolloverWillNot: "Not expected", RolloverCannot: "Unavailable because auto-renewal is disabled", RolloverUnavailable: "Temporarily unavailable",
		SignInReward: "Check-in reward", SignInAbove: "🎉 Congratulations! Your reward is above the average check-in reward.",
		SignInBelow: "🌱 Your reward is below the average check-in reward. Better luck tomorrow!", SignInEqual: "✨ Your reward matches the average check-in reward.",
		SignInNeutral: "✨ Check-in successful!", SignInAlready: "✅ You have already checked in today.",
		GroupBoostRequired:    "Check-in refused: you have not boosted the group yet. Add at least one active boost to receive activity rewards.",
		GroupBoostUnavailable: "Group boost verification is temporarily unavailable. Please try again.",
		DeductUsage:           "Reply to a member with /deduct <amount>.", DeductRejected: "The deduction could not be completed.", DeductSucceeded: "The balance was deducted.",
	}
}

func chineseCopy() Copy {
	return Copy{
		Descriptions: []Description{{Sub, "查看订阅用量"}, {Balance, "查看 TXB 余额"}, {SignIn, "领取今日签到奖励"}, {Start, "打开 VanSail Carpool"}, {MyCombo, "查看套餐与结转状态"}},
		Start:        "请按“打开 VanSail Carpool”使用应用。如需帮助，请先打开面板并完成首次访问验证，再直接在这里发送消息。", WelcomePrefix: "👋 欢迎 ", WelcomeSuffix: " 加入 VanSail Carpool！", MemberLabel: "新成员",
		UnknownTitle: "命令不可用", Unknown: "此命令不可用。",
		UnavailableTitle: "暂时不可用", Unavailable: "暂时无法获取所需信息。",
		NoSubscriptionTitle: "订阅", NoSubscription: "当前没有生效中的订阅。", SubscriptionTitle: "我的订阅", ComboTitle: "我的套餐", DeductTitle: "余额扣减",
		StatusLabel: "状态", AmountLabel: "金额", BalanceLabel: "余额",
		ComboLabel: "套餐", SquadsLabel: "线路组", TrafficLabel: "流量", ResetLabel: "重置",
		ResetDaily: "每日", ResetWeekly: "每周", ResetMonthly: "滚动月度", RolloverLabel: "结转",
		RolloverWill: "预计", RolloverWillNot: "预计无法获得", RolloverCannot: "未启用自动续费，无法获得", RolloverUnavailable: "暂不可用",
		SignInReward: "签到奖励", SignInAbove: "🎉 恭喜！本次奖励高于平均签到奖励。", SignInBelow: "🌱 本次奖励低于平均签到奖励，明天继续加油。",
		SignInEqual: "✨ 本次奖励正好达到平均签到奖励。", SignInNeutral: "✨ 签到成功！", SignInAlready: "✅ 今日已签到。",
		GroupBoostRequired:    "签到被拒绝：你尚未为群组助力。至少保持一次有效助力后才能获得活动奖励。",
		GroupBoostUnavailable: "暂时无法验证群组助力，请稍后再试。",
		DeductUsage:           "请回复一位成员的消息后使用 /deduct <金额>。", DeductRejected: "扣减未能完成。", DeductSucceeded: "余额已扣减。",
	}
}
