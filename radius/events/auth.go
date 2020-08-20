package events

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/meklis/all-ok-radius-server/redback_agent_parsers"
	"net"
	"time"
)

type AuthRequest struct {
	NasIp           string            `json:"nas_ip" yaml:"nas_ip"`
	NasName         string            `json:"nas_name" yaml:"nas_name"`
	DeviceMac       string            `json:"device_mac"`
	DhcpServerName  string            `json:"dhcp_server_name"`
	DhcpServerId    string            `json:"dhcp_server_id"`
	Agent           *AuthRequestAgent `json:"agent"`
	FramedIpAddress string            `json:"ip_address"`
	Class           string            `json:"class_id"`
}

type AuthRequestAgent struct {
	Circuit      *redback_agent_parsers.CircuitId `json:"circuit_id"`
	RemoteId     string                           `json:"remote_id"`
	RawCircuitId string                           `json:"_raw_circuit_id"`
}

func (r *AuthRequest) GetHash() string {
	arrBytes := []byte{}
	jsonBytes, _ := json.Marshal(r)
	arrBytes = append(arrBytes, jsonBytes...)
	return fmt.Sprintf("%x", md5.Sum(arrBytes))
}

type AuthResponse struct {
	Time         time.Time `json:"-"`
	IpAddress    string    `json:"ip_address"`
	PoolName     string    `json:"pool_name"`
	LeaseTimeSec int       `json:"lease_time_sec"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
	Class        string    `json:"class_id"`
}

type RadiusResponseType int

const SetPool RadiusResponseType = 1
const SetIpAddress RadiusResponseType = 2

func (r *AuthResponse) GetRadiusResponseType() RadiusResponseType {
	if r.IpAddress != "" {
		return SetIpAddress
	} else {
		return SetPool
	}
}
func (r *AuthResponse) GetIp() net.IP {
	return net.ParseIP(r.IpAddress)
}
