package events

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/meklis/all-ok-radius-server/redback_agent_parsers"
)

type Request struct {
	NasIp          string        `json:"nas_ip" yaml:"nas_ip"`
	NasName        string        `json:"nas_name" yaml:"nas_name"`
	DeviceMac      string        `json:"device_mac"`
	DhcpServerName string        `json:"dhcp_server_name"`
	DhcpServerId   string        `json:"dhcp_server_id"`
	Agent          *RequestAgent `json:"agent"`
}

type RequestAgent struct {
	Circuit      *redback_agent_parsers.CircuitId `json:"circuit_id"`
	RemoteId     string                           `json:"remote_id"`
	RawCircuitId string                           `json:"_raw_circuit_id"`
}

func (r *Request) GetHash() string {
	arrBytes := []byte{}
	jsonBytes, _ := json.Marshal(r)
	arrBytes = append(arrBytes, jsonBytes...)
	return fmt.Sprintf("%x", md5.Sum(arrBytes))
}
