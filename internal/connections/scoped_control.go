package connections

import "context"

// UserScopedIPRemote preserves ownership when several members share a source IP.
// Legacy providers can retain their existing API until their node is handed over.
type UserScopedIPRemote interface {
	ControlMemberIP(context.Context, string, string, string, bool, int, bool) error
}

func scopedControl(ctx context.Context, remote any, user, ip, node string, drop bool, seconds int, unblock bool) (bool, error) {
	provider, ok := remote.(UserScopedIPRemote)
	if !ok {
		return false, nil
	}
	return true, provider.ControlMemberIP(ctx, user, ip, node, drop, seconds, unblock)
}
