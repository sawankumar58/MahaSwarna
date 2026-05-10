package events

const ChannelFlagUpdated = "flag_updated"

type FlagUpdatedPayload struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
