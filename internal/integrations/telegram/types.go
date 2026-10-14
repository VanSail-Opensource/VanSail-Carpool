package telegram

// User is the subset of a Telegram Bot API user used by the application.
type User struct {
	ID            int64  `json:"id"`
	IsBot         bool   `json:"is_bot"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name,omitempty"`
	Username      string `json:"username,omitempty"`
	LanguageCode  string `json:"language_code,omitempty"`
	AllowsWritePM bool   `json:"allows_write_to_pm,omitempty"`
	PhotoURL      string `json:"photo_url,omitempty"`
}

// Chat is the subset of a Telegram chat needed for membership processing.
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// ChatInviteLink describes an invite link created by the bot.
type ChatInviteLink struct {
	InviteLink              string `json:"invite_link"`
	Creator                 User   `json:"creator"`
	CreatesJoinRequest      bool   `json:"creates_join_request"`
	IsPrimary               bool   `json:"is_primary"`
	IsRevoked               bool   `json:"is_revoked"`
	Name                    string `json:"name,omitempty"`
	ExpireDate              int64  `json:"expire_date,omitempty"`
	MemberLimit             int    `json:"member_limit,omitempty"`
	PendingJoinRequestCount int    `json:"pending_join_request_count,omitempty"`
}

// ChatJoinRequest is delivered when a user follows a join-request invite.
type ChatJoinRequest struct {
	Chat       Chat            `json:"chat"`
	From       User            `json:"from"`
	UserChatID int64           `json:"user_chat_id"`
	Date       int64           `json:"date"`
	Bio        string          `json:"bio,omitempty"`
	InviteLink *ChatInviteLink `json:"invite_link,omitempty"`
	QueryID    string          `json:"query_id,omitempty"`
}

// ChatMember is the common portion of Telegram's chat-member variants.
type ChatMember struct {
	Status          string `json:"status"`
	User            User   `json:"user"`
	IsMember        bool   `json:"is_member,omitempty"`
	CustomTitle     string `json:"custom_title,omitempty"`
	Tag             string `json:"tag,omitempty"`
	CanManageTags   bool   `json:"can_manage_tags,omitempty"`
	CanEditTag      bool   `json:"can_edit_tag,omitempty"`
	CanManageTopics bool   `json:"can_manage_topics,omitempty"`
	UntilDate       int64  `json:"until_date,omitempty"`
}

// Present reports whether a chat-member response represents current membership.
func (m ChatMember) Present() bool {
	switch m.Status {
	case "creator", "administrator", "member":
		return true
	case "restricted":
		return m.IsMember
	default:
		return false
	}
}

// ChatMemberUpdated describes a transition in a user's chat membership.
type ChatMemberUpdated struct {
	Chat          Chat            `json:"chat"`
	From          User            `json:"from"`
	Date          int64           `json:"date"`
	OldChatMember ChatMember      `json:"old_chat_member"`
	NewChatMember ChatMember      `json:"new_chat_member"`
	InviteLink    *ChatInviteLink `json:"invite_link,omitempty"`
}

// SuccessfulPayment contains the fields used to authoritatively settle a Stars order.
type SuccessfulPayment struct {
	Currency                   string `json:"currency"`
	TotalAmount                int64  `json:"total_amount"`
	InvoicePayload             string `json:"invoice_payload"`
	SubscriptionExpirationDate int64  `json:"subscription_expiration_date,omitempty"`
	IsRecurring                bool   `json:"is_recurring,omitempty"`
	IsFirstRecurring           bool   `json:"is_first_recurring,omitempty"`
	TelegramPaymentChargeID    string `json:"telegram_payment_charge_id"`
	ProviderPaymentChargeID    string `json:"provider_payment_charge_id"`
}

// RefundedPayment describes a Telegram Stars refund service message.
type RefundedPayment struct {
	Currency                string `json:"currency"`
	TotalAmount             int64  `json:"total_amount"`
	InvoicePayload          string `json:"invoice_payload"`
	TelegramPaymentChargeID string `json:"telegram_payment_charge_id"`
	ProviderPaymentChargeID string `json:"provider_payment_charge_id,omitempty"`
}

// PreCheckoutQuery is sent before Telegram completes a payment.
type PreCheckoutQuery struct {
	ID             string `json:"id"`
	From           User   `json:"from"`
	Currency       string `json:"currency"`
	TotalAmount    int64  `json:"total_amount"`
	InvoicePayload string `json:"invoice_payload"`
}

// Update is the webhook envelope for the update types VanSail Carpool subscribes to.
type Update struct {
	UpdateID         int64              `json:"update_id"`
	Message          *Message           `json:"message,omitempty"`
	EditedMessage    *Message           `json:"edited_message,omitempty"`
	ChatMember       *ChatMemberUpdated `json:"chat_member,omitempty"`
	MyChatMember     *ChatMemberUpdated `json:"my_chat_member,omitempty"`
	ChatJoinRequest  *ChatJoinRequest   `json:"chat_join_request,omitempty"`
	PreCheckoutQuery *PreCheckoutQuery  `json:"pre_checkout_query,omitempty"`
	ChatBoost        *ChatBoostUpdated  `json:"chat_boost,omitempty"`
	CallbackQuery    *CallbackQuery     `json:"callback_query,omitempty"`
}

// LabeledPrice is a Telegram invoice line item. Stars invoices must contain one.
type LabeledPrice struct {
	Label  string `json:"label"`
	Amount int64  `json:"amount"`
}

// StarTransaction is the stable identity and amount portion of a bot Stars transaction.
type StarTransaction struct {
	ID             string                  `json:"id"`
	Amount         int64                   `json:"amount"`
	NanostarAmount int64                   `json:"nanostar_amount,omitempty"`
	Date           int64                   `json:"date"`
	Source         *TransactionPartnerUser `json:"source,omitempty"`
	Receiver       *TransactionPartnerUser `json:"receiver,omitempty"`
}

// TransactionPartnerUser is the invoice-originating user subset needed for reconciliation.
type TransactionPartnerUser struct {
	Type            string `json:"type"`
	TransactionType string `json:"transaction_type"`
	User            User   `json:"user"`
	InvoicePayload  string `json:"invoice_payload,omitempty"`
}
