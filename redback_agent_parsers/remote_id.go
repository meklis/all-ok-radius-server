package redback_agent_parsers

import (
	"fmt"
	"strings"
)

func ParseRemoteId(remoteIdBytes []byte) string {
	if remoteIdBytes == nil || len(remoteIdBytes) <= 2 {
		return ""
	}
	var remoteId string
	for _, h_block := range remoteIdBytes[2:] {
		h := fmt.Sprintf("%X", h_block)
		if len(h) == 1 {
			h = "0" + h
		}
		remoteId += h + ":"
	}
	remoteId = strings.Trim(remoteId, ":")
	return remoteId
}
