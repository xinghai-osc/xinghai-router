package app

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// sessionAffinityLabels resolves the display names of the users, groups,
// channels and channel credentials referenced by a page of assignments. The
// references may point at rows deleted since they were pinned, so every join is
// optional and a missing label is simply left empty.
type sessionAffinityLabels struct {
	users    map[string]string
	groups   map[string]string
	channels map[int64]string
	keys     map[string]string
}

func (l sessionAffinityLabels) userName(userID string) string {
	return l.users[userID]
}

func (l sessionAffinityLabels) groupName(groupID string) string {
	return l.groups[groupID]
}

func (l sessionAffinityLabels) channelName(channelID int64) string {
	return l.channels[channelID]
}

func (l sessionAffinityLabels) keyName(keyID string) string {
	return l.keys[keyID]
}

// listSessionAffinity returns the live conversation assignments of this
// process, newest expiry last, so an operator can see which channel each
// conversation is pinned to and drop a pin that has become wrong - a channel
// that serves a model badly, a conversation that must be rebalanced, or an
// assignment created before a channel was reconfigured.
func (s *Service) listSessionAffinity(w http.ResponseWriter, r *http.Request) {
	if s.sessionAffinity == nil {
		writePaged(w, []map[string]any{}, 0, 1, 50)
		return
	}
	query := r.URL.Query()
	session := strings.TrimSpace(query.Get("session"))
	userID := strings.TrimSpace(query.Get("user_id"))
	model := strings.TrimSpace(query.Get("model"))
	channelID := strings.TrimSpace(query.Get("channel_id"))
	page, pageSize, offset := listPage(r)

	records := s.sessionAffinity.list()
	filtered := make([]sessionAffinityRecord, 0, len(records))
	for _, record := range records {
		if session != "" && !strings.Contains(record.Session, session) {
			continue
		}
		if userID != "" && record.UserID != userID {
			continue
		}
		if model != "" && record.Model != model {
			continue
		}
		if channelID != "" && strconv.FormatInt(record.ChannelID, 10) != channelID {
			continue
		}
		filtered = append(filtered, record)
	}
	// Longest lease first: a refreshed conversation is the one still in use.
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ExpiresAt.After(filtered[j].ExpiresAt) })

	total := len(filtered)
	switch {
	case offset >= total:
		filtered = nil
	case total-offset <= pageSize:
		filtered = filtered[offset:]
	default:
		filtered = filtered[offset : offset+pageSize]
	}

	labels := s.sessionAffinityLabelMap(r.Context(), filtered)
	data := make([]map[string]any, 0, len(filtered))
	for _, record := range filtered {
		data = append(data, map[string]any{
			"session":          record.Session,
			"session_prefix":   shortSessionPrefix(record.Session),
			"session_length":   len(record.Session),
			"user_id":          record.UserID,
			"user_name":        labels.userName(record.UserID),
			"group_id":         record.GroupID,
			"group_name":       labels.groupName(record.GroupID),
			"model":            record.Model,
			"channel_id":       record.ChannelID,
			"channel_name":     labels.channelName(record.ChannelID),
			"channel_key_id":   record.KeyID,
			"channel_key_name": labels.keyName(record.KeyID),
			"expires_at":       record.ExpiresAt,
		})
	}
	writePaged(w, data, total, page, pageSize)
}

// deleteSessionAffinity drops every assignment of one session id, narrowed to
// one user when the request carries a user id. The gateway tries the remaining
// candidates in their own order from the next request on, which is the
// operator's escape hatch when an assignment is wrong.
func (s *Service) deleteSessionAffinity(w http.ResponseWriter, r *http.Request) {
	if s.sessionAffinity == nil {
		writeError(w, http.StatusServiceUnavailable, "affinity_disabled", "session affinity is not available")
		return
	}
	session := strings.TrimSpace(r.PathValue("session"))
	if session == "" || len(session) > maxSessionIDLength {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid session id is required")
		return
	}
	query := r.URL.Query()
	userID := strings.TrimSpace(query.Get("user_id"))
	removed := 0
	if userID == "" {
		removed = s.sessionAffinity.purgeSession(session)
	} else {
		for _, record := range s.sessionAffinity.list() {
			if record.Session != session || record.UserID != userID {
				continue
			}
			if s.sessionAffinity.purge(sessionAffinityKey{session: record.Session, userID: record.UserID, groupID: record.GroupID, model: record.Model}) {
				removed++
			}
		}
	}
	if removed == 0 {
		writeError(w, http.StatusNotFound, "not_found", "no session affinity assignment for this session")
		return
	}
	s.audit(r, "session_affinity.deleted", "session_affinity", session, map[string]any{"removed": removed, "user_id": userID})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": removed})
}

// clearSessionAffinity drops every assignment, or every assignment of one user
// when the request carries a user id, and reports how many were removed.
func (s *Service) clearSessionAffinity(w http.ResponseWriter, r *http.Request) {
	if s.sessionAffinity == nil {
		writeError(w, http.StatusServiceUnavailable, "affinity_disabled", "session affinity is not available")
		return
	}
	var in struct {
		UserID string `json:"user_id"`
	}
	if r.Body != nil {
		_ = decode(r, &in)
	}
	userID := strings.TrimSpace(in.UserID)
	removed := 0
	if userID == "" {
		removed = s.sessionAffinity.clear()
	} else {
		removed = s.sessionAffinity.purgeUser(userID)
	}
	if removed > 0 {
		s.audit(r, "session_affinity.cleared", "session_affinity", "all", map[string]any{"removed": removed, "user_id": userID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": removed})
}

// shortSessionPrefix bounds the session id an admin list renders: a derived
// conversation reference is a 32-character hash and adds nothing to the view
// beyond its head, while a client-declared session id is usually short enough
// to be shown whole.
func shortSessionPrefix(session string) string {
	const prefixLength = 16
	if len(session) <= prefixLength {
		return session
	}
	return session[:prefixLength]
}

func (s *Service) sessionAffinityLabelMap(ctx context.Context, records []sessionAffinityRecord) sessionAffinityLabels {
	labels := sessionAffinityLabels{
		users:    map[string]string{},
		groups:   map[string]string{},
		channels: map[int64]string{},
		keys:     map[string]string{},
	}
	userIDs, groupIDs, keyIDs := []string{}, []string{}, []string{}
	channelIDs := []int64{}
	seenUser, seenGroup, seenChannel, seenKey := map[string]bool{}, map[string]bool{}, map[int64]bool{}, map[string]bool{}
	for _, record := range records {
		if record.UserID != "" && !seenUser[record.UserID] {
			seenUser[record.UserID] = true
			userIDs = append(userIDs, record.UserID)
		}
		if record.GroupID != "" && !seenGroup[record.GroupID] {
			seenGroup[record.GroupID] = true
			groupIDs = append(groupIDs, record.GroupID)
		}
		if record.ChannelID != 0 && !seenChannel[record.ChannelID] {
			seenChannel[record.ChannelID] = true
			channelIDs = append(channelIDs, record.ChannelID)
		}
		if record.KeyID != "" && !seenKey[record.KeyID] {
			seenKey[record.KeyID] = true
			keyIDs = append(keyIDs, record.KeyID)
		}
	}
	if len(userIDs) > 0 {
		if rows, err := s.db.Query(ctx, `select id::text,coalesce(name,''),coalesce(email,'') from users where id = any($1::bigint[])`, intIDs(userIDs)); err == nil {
			for rows.Next() {
				var id, name, email string
				if rows.Scan(&id, &name, &email) != nil {
					continue
				}
				if name == "" {
					name = email
				}
				labels.users[id] = name
			}
			rows.Close()
		}
	}
	if len(groupIDs) > 0 {
		if rows, err := s.db.Query(ctx, `select id::text,coalesce(nullif(display_name,''),name,'') from groups where id::text = any($1::text[])`, groupIDs); err == nil {
			for rows.Next() {
				var id, name string
				if rows.Scan(&id, &name) != nil {
					continue
				}
				labels.groups[id] = name
			}
			rows.Close()
		}
	}
	if len(channelIDs) > 0 {
		if rows, err := s.db.Query(ctx, `select id,coalesce(name,'') from channels where id = any($1::bigint[])`, channelIDs); err == nil {
			for rows.Next() {
				var id int64
				var name string
				if rows.Scan(&id, &name) != nil {
					continue
				}
				labels.channels[id] = name
			}
			rows.Close()
		}
	}
	if len(keyIDs) > 0 {
		if rows, err := s.db.Query(ctx, `select id::text,coalesce(name,'') from channel_api_keys where id::text = any($1::text[])`, keyIDs); err == nil {
			for rows.Next() {
				var id, name string
				if rows.Scan(&id, &name) != nil {
					continue
				}
				labels.keys[id] = name
			}
			rows.Close()
		}
	}
	return labels
}
