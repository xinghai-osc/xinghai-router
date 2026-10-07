package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type batchUserUpdateInput struct {
	UserIDs             []string        `json:"user_ids"`
	Enabled             *bool           `json:"enabled"`
	Role                *string         `json:"role"`
	Permissions         *[]string       `json:"permissions"`
	Groups              *[]string       `json:"groups"`
	LeaderboardOptIn    *bool           `json:"leaderboard_opt_in"`
	LeaderboardMaskName *bool           `json:"leaderboard_mask_name"`
	DataUsageEnabled    *bool           `json:"data_usage_enabled"`
	MaxConcurrency      json.RawMessage `json:"max_concurrency"`
}

type normalizedBatchUserUpdate struct {
	userIDs             []string
	enabled             *bool
	role                *string
	permissions         []string
	permissionsSet      bool
	groupRefs           []string
	groupsSet           bool
	leaderboardOptIn    *bool
	leaderboardMaskName *bool
	dataUsageEnabled    *bool
	maxConcurrency      *int
	maxConcurrencySet   bool
}

func normalizeBatchUserUpdate(in batchUserUpdateInput) (normalizedBatchUserUpdate, error) {
	if len(in.UserIDs) == 0 {
		return normalizedBatchUserUpdate{}, errors.New("a non-empty list of user ids is required")
	}
	if len(in.UserIDs) > 100 {
		return normalizedBatchUserUpdate{}, errors.New("at most 100 users can be updated at a time")
	}

	ids := make([]string, 0, len(in.UserIDs))
	seenIDs := make(map[string]bool, len(in.UserIDs))
	for _, raw := range in.UserIDs {
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || value <= 0 || value > maxEditableUserID {
			return normalizedBatchUserUpdate{}, errors.New("every user id must be a positive integer up to 9007199254740991")
		}
		id := strconv.FormatInt(value, 10)
		if seenIDs[id] {
			return normalizedBatchUserUpdate{}, errors.New("user ids must be unique")
		}
		seenIDs[id] = true
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, _ := strconv.ParseInt(ids[i], 10, 64)
		right, _ := strconv.ParseInt(ids[j], 10, 64)
		return left < right
	})

	update := normalizedBatchUserUpdate{userIDs: ids, enabled: in.Enabled, groupsSet: in.Groups != nil, leaderboardOptIn: in.LeaderboardOptIn, leaderboardMaskName: in.LeaderboardMaskName, dataUsageEnabled: in.DataUsageEnabled}
	if in.Role != nil {
		role := strings.TrimSpace(*in.Role)
		if role != "user" && role != "operator" && role != "admin" {
			return normalizedBatchUserUpdate{}, errors.New("role must be user, operator, or admin")
		}
		update.role = &role
	}
	if in.Permissions != nil {
		update.permissionsSet = true
		seenPermissions := map[string]bool{}
		for _, permission := range *in.Permissions {
			if !availablePermissions[permission] || seenPermissions[permission] {
				return normalizedBatchUserUpdate{}, errors.New("invalid permissions")
			}
			seenPermissions[permission] = true
			update.permissions = append(update.permissions, permission)
		}
	}
	if in.Groups != nil {
		update.groupRefs = make([]string, 0, len(*in.Groups))
		seenGroups := map[string]bool{}
		for _, raw := range *in.Groups {
			ref := strings.TrimSpace(raw)
			if ref == "" || seenGroups[ref] {
				continue
			}
			seenGroups[ref] = true
			update.groupRefs = append(update.groupRefs, ref)
		}
	}
	if len(in.MaxConcurrency) > 0 {
		update.maxConcurrencySet = true
		raw := strings.TrimSpace(string(in.MaxConcurrency))
		if raw != "null" {
			var value int
			if json.Unmarshal(in.MaxConcurrency, &value) != nil || value <= 0 || value > 10000 {
				return normalizedBatchUserUpdate{}, errors.New("max_concurrency must be between 1 and 10000, or null")
			}
			update.maxConcurrency = &value
		}
	}
	if update.enabled == nil && update.role == nil && !update.permissionsSet && !update.groupsSet && update.leaderboardOptIn == nil && update.leaderboardMaskName == nil && update.dataUsageEnabled == nil && !update.maxConcurrencySet {
		return normalizedBatchUserUpdate{}, errors.New("at least one user field is required")
	}
	return update, nil
}

func (s *Service) batchUpdateUsers(w http.ResponseWriter, r *http.Request) {
	body, err := readRequestBody(w, r, s.cfg.RequestBodyTimeout)
	if err != nil {
		status, code, message := requestBodyError(err)
		writeError(w, status, code, message)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var in batchUserUpdateInput
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid user batch update")
		return
	}
	update, err := normalizeBatchUserUpdate(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database unavailable")
		return
	}

	actor := accountFromContext(r)
	mutation := userMutation{authorization: update.role != nil || update.permissionsSet, groups: update.groupsSet}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update users")
		return
	}
	defer tx.Rollback(r.Context())

	actor, _, err = lockUserMutation(r.Context(), tx, actor.userID, "", mutation, true)
	if err != nil {
		writeUserMutationError(w, err)
		return
	}

	resolvedGroups := []string{}
	if update.groupsSet {
		resolved := map[string]bool{}
		for _, ref := range update.groupRefs {
			var id string
			if err = tx.QueryRow(r.Context(), `select id from groups where id::text=$1 or name=$2`, ref, ref).Scan(&id); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "unknown group")
				return
			}
			resolved[id] = true
		}
		resolvedGroups = sortedKeys(resolved)
	}

	type targetUser struct {
		access          userAccess
		resultingRole   string
		resultingEnable bool
	}
	targets := make([]targetUser, 0, len(update.userIDs))
	for _, userID := range update.userIDs {
		target, lockErr := lockUserAccess(r.Context(), tx, userID)
		if lockErr != nil {
			writeUserMutationError(w, lockErr)
			return
		}
		if validateErr := validateUserMutation(actor, &target, mutation, true); validateErr != nil {
			writeUserMutationError(w, validateErr)
			return
		}
		resultingRole := target.role
		if update.role != nil {
			resultingRole = *update.role
		}
		resultingEnabled := target.enabled
		if update.enabled != nil {
			resultingEnabled = *update.enabled
		}
		if actor.userID == target.userID && !resultingEnabled {
			writeError(w, http.StatusBadRequest, "invalid_request", "cannot disable your own account")
			return
		}
		targets = append(targets, targetUser{access: target, resultingRole: resultingRole, resultingEnable: resultingEnabled})
	}

	if update.role != nil || update.enabled != nil {
		var enabledAdmins int
		if err = tx.QueryRow(r.Context(), `select count(*) from users where role='admin' and enabled`).Scan(&enabledAdmins); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not validate administrators")
			return
		}
		for _, target := range targets {
			if target.access.role == "admin" && target.access.enabled {
				enabledAdmins--
			}
			if target.resultingRole == "admin" && target.resultingEnable {
				enabledAdmins++
			}
		}
		if enabledAdmins < 1 {
			writeUserMutationError(w, errLastAdministrator)
			return
		}
	}

	for _, target := range targets {
		userID := target.access.userID
		if update.role != nil {
			if _, err = tx.Exec(r.Context(), `update users set role=$1 where id=$2`, *update.role, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update role")
				return
			}
			if target.access.role != *update.role {
				if _, err = tx.Exec(r.Context(), `delete from user_sessions where user_id=$1`, userID); err != nil {
					writeError(w, http.StatusInternalServerError, "internal_error", "could not revoke sessions after role change")
					return
				}
			}
		}
		if update.permissionsSet {
			if _, err = tx.Exec(r.Context(), `delete from user_permissions where user_id=$1`, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
				return
			}
			for _, permission := range update.permissions {
				if _, err = tx.Exec(r.Context(), `insert into user_permissions(user_id,permission) values($1,$2)`, userID, permission); err != nil {
					writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
					return
				}
			}
			if _, err = tx.Exec(r.Context(), `delete from user_sessions where user_id=$1`, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not revoke sessions after permissions change")
				return
			}
		}
		if update.enabled != nil {
			if *update.enabled {
				if shouldReleaseRewardBan(target.access.enabled, *update.enabled) {
					if err = s.releaseRewardBanTx(r.Context(), tx, userID, actor.userID, "Administrator enabled account in batch"); err != nil {
						writeError(w, http.StatusInternalServerError, "internal_error", "could not release reward ban")
						return
					}
				}
				if _, err = tx.Exec(r.Context(), `update users set enabled=true where id=$1`, userID); err != nil {
					writeError(w, http.StatusInternalServerError, "internal_error", "could not update status")
					return
				}
			} else if err = disableUserAccessTx(r.Context(), tx, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not disable user access")
				return
			}
		}
		if update.leaderboardOptIn != nil {
			if _, err = tx.Exec(r.Context(), `update users set leaderboard_opt_in=$1 where id=$2`, *update.leaderboardOptIn, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update leaderboard opt-in")
				return
			}
		}
		if update.leaderboardMaskName != nil {
			if _, err = tx.Exec(r.Context(), `update users set leaderboard_mask_name=$1 where id=$2`, *update.leaderboardMaskName, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update leaderboard name masking")
				return
			}
		}
		if update.dataUsageEnabled != nil {
			if _, err = tx.Exec(r.Context(), `update users set data_usage_enabled=$1 where id=$2`, *update.dataUsageEnabled, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update data usage setting")
				return
			}
		}
		if update.maxConcurrencySet {
			if _, err = tx.Exec(r.Context(), `update users set max_concurrency=$1 where id=$2`, update.maxConcurrency, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update concurrency limit")
				return
			}
		}
		if update.groupsSet {
			if _, err = tx.Exec(r.Context(), `delete from user_groups where user_id=$1`, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update groups")
				return
			}
			for _, groupID := range resolvedGroups {
				if _, err = tx.Exec(r.Context(), `insert into user_groups(user_id,group_id) values($1,$2)`, userID, groupID); err != nil {
					writeError(w, http.StatusInternalServerError, "internal_error", "could not update groups")
					return
				}
			}
			if _, err = tx.Exec(r.Context(), `update api_keys set group_id=null where user_id=$1 and group_id is not null and not exists(select 1 from user_groups ug where ug.user_id=$1 and ug.group_id=api_keys.group_id)`, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not update API key groups")
				return
			}
		}
	}

	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update users")
		return
	}

	changed := map[string]any{"count": len(targets), "user_ids": update.userIDs}
	if update.enabled != nil {
		changed["enabled"] = *update.enabled
	}
	if update.role != nil {
		changed["role"] = *update.role
	}
	if update.permissionsSet {
		changed["permissions"] = update.permissions
	}
	if update.groupsSet {
		changed["groups"] = resolvedGroups
	}
	if update.maxConcurrencySet {
		changed["max_concurrency"] = update.maxConcurrency
	}
	if update.leaderboardOptIn != nil {
		changed["leaderboard_opt_in"] = *update.leaderboardOptIn
	}
	if update.leaderboardMaskName != nil {
		changed["leaderboard_mask_name"] = *update.leaderboardMaskName
	}
	if update.dataUsageEnabled != nil {
		changed["data_usage_enabled"] = *update.dataUsageEnabled
	}
	s.audit(r, "users.batch_updated", "user", "batch", changed)
	if update.maxConcurrencySet {
		for _, userID := range update.userIDs {
			s.userConcurrencyCache.invalidate(userID)
		}
	}
	if update.groupsSet {
		s.invalidateChannels()
	}
	writeJSON(w, http.StatusOK, map[string]any{"affected": len(targets), "user_ids": update.userIDs})
}
