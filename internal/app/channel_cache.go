package app

// channelRouteKey identifies the channel list a gateway request needs. When a
// key is bound to a group (groupID != "") the query result no longer depends on
// the user, so the key is normalized to (groupID, model) and every user in the
// group shares one cache entry. Ungrouped keys are keyed per (userID, model).
type channelRouteKey struct {
	userID  string
	groupID string
	model   string
}

// subscriptionRouteKey identifies a user's subscription coverage answer for one
// model. Coverage depends only on the user and the requested model.
type subscriptionRouteKey struct {
	userID string
	model  string
}

// invalidateChannels drops the cached channel lists and keys. It is called after
// any channel, channel key, model route, or group-membership write so the gateway
// picks up configuration changes immediately instead of waiting out the TTL.
func (s *Service) invalidateChannels() {
	s.channelCache.clear()
	s.channelKeyCache.clear()
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
