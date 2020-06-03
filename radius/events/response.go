package events

import (
	"net"
	"time"
)

type Response struct {
	Time         time.Time `json:"-"`
	IpAddress    string    `json:"ip_address"`
	PoolName     string    `json:"pool_name"`
	LeaseTimeSec int       `json:"lease_time_sec"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
}

type RadiusResponseType int

const SetPool RadiusResponseType = 1
const SetIpAddress RadiusResponseType = 2

func (r *Response) GetRadiusResponseType() RadiusResponseType {
	if r.IpAddress != "" {
		return SetIpAddress
	} else {
		return SetPool
	}
}
func (r *Response) GetIp() net.IP {
	return net.ParseIP(r.IpAddress)
}
