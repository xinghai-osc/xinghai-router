package app

// channelRouteKey identifies the channel list a gateway request needs. A channel
// may be restricted to a single user (channels.user_id), so the candidate list
// depends on the caller's identity even when the key is bound to a group; the
// user part of the key is therefore never normalized away.
type channelRouteKey struct {
	userID  string
	groupID string
	model   string
}

// subscriptionRouteKey identifies a user's subscription coverage answer for one
// model. Coverage is scoped to the key's group: only subscriptions whose plan is
// bound to that group (or to no group for an ungrouped key) may be consumed.
type subscriptionRouteKey struct {
	userID  string
	groupID string
	model   string
}

// quotaRouteKey identifies a key's quota absence status. The cache is used only
// for the common case where a key has no quota limits configured, so the
// per-request aggregation query over request_logs can be skipped entirely.
type quotaRouteKey struct {
	userID string
	keyID  string
	model  string
}

// invalidateChannels drops the cached channel lists and keys. It is called after
// any channel, channel key, model route, or group-membership write so the gateway
// picks up configuration changes immediately instead of waiting out the TTL.
func (s *Service) invalidateChannels() {
	s.channelCache.clear()
	s.channelKeyCache.clear()
}

// invalidateQuotaAbsence drops the cached "key has no quota" answers. It is
// called after any quota_limits write so newly-configured quotas take effect
// immediately instead of waiting out the TTL.
func (s *Service) invalidateQuotaAbsence() {
	if s.quotaAbsentCache != nil {
		s.quotaAbsentCache.clear()
	}
}

// invalidateChannelQuota drops the cached channel quota verdicts. It is called
// after any channel_quota_limits write so newly-configured channel quotas take
// effect immediately instead of waiting out the TTL.
func (s *Service) invalidateChannelQuota() {
	if s.channelQuotaCache != nil {
		s.channelQuotaCache.clear()
	}
}

// cloneChannels returns a shallow copy of a cached channel list. The retry loop
// rotates credentials by mutating the channel struct, so every request must work
// on its own copy rather than the shared cache entry.
func cloneChannels(in []channel) []channel {
	out := make([]channel, len(in))
	copy(out, in)
	return out
}

func cloneChannelKeys(in []channelKeyCredential) []channelKeyCredential {
	out := make([]channelKeyCredential, len(in))
	copy(out, in)
	return out
}
