package events

type AcctRequest struct {
	NasIp           string `json:"nas_ip" yaml:"nas_ip"`
	NasName         string `json:"nas_name" yaml:"nas_name"`
	DeviceMac       string `json:"device_mac"`
	DhcpServerName  string `json:"dhcp_server_name"`
	DhcpServerId    string `json:"dhcp_server_id"`
	FramedIpAddress string `json:"ip_address"`
	AuthType        string `json:"auth_type"`
	Class           string `json:"class_id"`
	StatusType      string `json:"status_type"`
	SessionTime     int64  `json:"session_time"`
	TerminateCause  string `json:"terminate_cause"`
	InputOctets     int64  `json:"input_octets"`
	OutputOctets    int64  `json:"output_octets"`
	PoolName        string `json:"pool_name"`
	SessionId       string `json:"session_id"`
}
