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
