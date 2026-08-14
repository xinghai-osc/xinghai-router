package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

const invitationAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newInvitationCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = invitationAlphabet[int(b[i])%len(invitationAlphabet)]
	}
	return string(b), nil
}

func (s *Service) ensureInvitationCode(ctx context.Context, tx pgx.Tx, userID string) (string, error) {
	var code string
	if err := tx.QueryRow(ctx, `select code from invitation_codes where user_id=$1`, userID).Scan(&code); err == nil {
		return code, nil
	} else if err != pgx.ErrNoRows {
		return "", err
	}
	for range 5 {
		generated, err := newInvitationCode()
		if err != nil {
			return "", err
		}
		err = tx.QueryRow(ctx, `insert into invitation_codes(user_id,code) values($1,$2) on conflict do nothing returning code`, userID, generated).Scan(&code)
		if err == nil {
			return code, nil
		}
		if err != pgx.ErrNoRows {
			return "", err
		}
		if err = tx.QueryRow(ctx, `select code from invitation_codes where user_id=$1`, userID).Scan(&code); err == nil {
			return code, nil
		} else if err != pgx.ErrNoRows {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate a unique invitation code")
}

func (s *Service) accountInvitations(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not load invitations")
		return
	}
	defer tx.Rollback(r.Context())
	code, err := s.ensureInvitationCode(r.Context(), tx, account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load invitation code")
		return
	}
	var enabled bool
	var inviterReward, inviteeReward string
	if err = tx.QueryRow(r.Context(), `select invitations_enabled,inviter_reward::text,invitee_reward::text from site_settings where id=true`).Scan(&enabled, &inviterReward, &inviteeReward); err != nil {
		writeError(w, 500, "internal_error", "could not load invitation settings")
		return
	}
	rows, err := tx.Query(r.Context(), `select i.id,u.name,u.email,i.inviter_reward::text,i.created_at from invitations i join users u on u.id=i.invitee_id where i.inviter_id=$1 order by i.created_at desc limit 100`, account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load invitations")
		return
	}
	defer rows.Close()
	data := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, email, reward string
		var created any
		if rows.Scan(&id, &name, &email, &reward, &created) == nil {
			data = append(data, map[string]any{"id": id, "name": name, "email": maskInvitationEmail(email), "reward": reward, "created_at": created})
		}
	}
	if err = rows.Err(); err != nil {
		writeError(w, 500, "internal_error", "could not load invitations")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not load invitations")
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": enabled, "code": code, "inviter_reward": inviterReward, "invitee_reward": inviteeReward, "data": data})
}

func maskInvitationEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	return string([]rune(parts[0])[0]) + "***@" + parts[1]
}
