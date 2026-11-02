package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeCustomizations(ctx context.Context, node string) ([]xnode.Customization, error) {
	return s.xnodeCustomizations(ctx, `WHERE node_id=$1 ORDER BY proxy_user_id`, node)
}

func (s *Store) XNodeAllCustomizations(ctx context.Context) ([]xnode.Customization, error) {
	return s.xnodeCustomizations(ctx, `ORDER BY node_id,proxy_user_id`)
}

func (s *Store) xnodeCustomizations(ctx context.Context, clause string, args ...any) ([]xnode.Customization, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT proxy_user_id,COALESCE(user_id,''),node_id,outbounds_cipher,routing_cipher,revision FROM xnode_customizations `+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Customization{}
	for rows.Next() {
		var item xnode.Customization
		if err = rows.Scan(&item.ProxyUserID, &item.UserID, &item.NodeID, &item.OutboundsCipher, &item.RoutingCipher, &item.Revision); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SaveXNodeCustomization(ctx context.Context, item xnode.Customization) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if item.ProxyUserID == 0 {
		identity, err := s.XNodeIdentityByUser(ctx, item.UserID)
		if err != nil {
			return err
		}
		item.ProxyUserID = identity.ProxyUserID
	}
	if item.Revision == 0 {
		_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_customizations(proxy_user_id,user_id,node_id,outbounds_cipher,routing_cipher,revision,updated_at) VALUES($1,$2,$3,$4,$5,1,$6)`, item.ProxyUserID, nullText(item.UserID), item.NodeID, item.OutboundsCipher, item.RoutingCipher, stamp(time.Now().UTC()))
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE xnode_customizations SET outbounds_cipher=$1,routing_cipher=$2,revision=revision+1,updated_at=$3 WHERE proxy_user_id=$4 AND node_id=$5 AND revision=$6`, item.OutboundsCipher, item.RoutingCipher, stamp(time.Now().UTC()), item.ProxyUserID, item.NodeID, item.Revision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return xnode.ErrConflict
	}
	return nil
}
