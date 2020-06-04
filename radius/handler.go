package radius

import (
	"errors"
	"fmt"
	"github.com/meklis/all-ok-radius-server/api"
	"github.com/meklis/all-ok-radius-server/prom"
	"github.com/meklis/all-ok-radius-server/radius/events"
	"github.com/meklis/all-ok-radius-server/redback"
	"github.com/meklis/all-ok-radius-server/redback_agent_parsers"
	"github.com/ztrue/tracerr"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
)

func (rad *Radius) handler(w radius.ResponseWriter, r *radius.Request) {
	req, _ := rad._handlerReadRequest(r)
	resp, err := rad._handlerProccessApi(req)

	if err != nil {
		prom.ErrorsInc(prom.Critical, "radius")
		rad.lg.CriticalF("error get answer from api's: %v", err.Error())
		rad.lg.DebugF(tracerr.Sprint(err))
		rad._handlerWriteResponseRejected(w, r)
		rad.api.SendPostAuth(api.PostAuth{
			Request: req,
			Response: events.Response{
				Status: "REJECT",
				Error:  fmt.Sprintf("%v", err),
			},
		})
		return
	} else if resp.IpAddress == "" && resp.PoolName == "" {
		prom.ErrorsInc(prom.Critical, "radius")
		rad.lg.CriticalF("error get answer from api's: pool_name and ip_address is empty")
		rad._handlerWriteResponseRejected(w, r)
		resp.Status = "REJECTED"
		resp.Error = fmt.Sprintf("error get answer from api's: pool_name and ip_address is empty")
		rad.api.SendPostAuth(api.PostAuth{
			Request:  req,
			Response: *resp,
		})
		return
	}
	prom.RadRequestsInc(req.NasIp)
	if resp.PoolName != "" {
		prom.RadRequestsPoolInc(req.NasIp)
	}
	if resp.IpAddress != "" {
		prom.RadRequestsIpAddressInc(req.NasIp)
	}
	rad.lg.DebugF("%v %x: response from api - poolName=%v ipAddr=%v leaseTimeSec=%v", r.Code, r.Authenticator, resp.PoolName, resp.IpAddress, resp.LeaseTimeSec)
	err = rad._handlerWriteResponse(*resp, w, r)
	if err != nil {
		rad._handlerWriteResponseRejected(w, r)
		resp.Status = "REJECTED"
		resp.Error = fmt.Sprintf("error write response: %v", err.Error())
		rad.api.SendPostAuth(api.PostAuth{
			Request:  req,
			Response: *resp,
		})
		prom.ErrorsInc(prom.Critical, "radius")
		rad.lg.CriticalF("error write response: %v", err.Error())
		rad.lg.DebugF(tracerr.Sprint(err))
		return
	}
	resp.Status = "ACCEPT"
	rad.api.SendPostAuth(api.PostAuth{
		Request:  req,
		Response: *resp,
	})
}

func (rad *Radius) _handlerProccessApi(request events.Request) (*events.Response, error) {
	resp, err := rad.api.Get(&request)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return resp, nil
}

func (rad *Radius) _handlerReadRequest(r *radius.Request) (events.Request, error) {
	nasName := rfc2865.NASIdentifier_GetString(r.Packet)
	nasIpAddr := rfc2865.NASIPAddress_Get(r.Packet).String()
	deviceMAC := rfc2865.UserName_GetString(r.Packet)
	dhcpServerName := rfc2865.CalledStationID_GetString(r.Packet)
	dhcpServerId := rfc2865.CallingStationID_GetString(r.Packet)

	rad.lg.DebugF("%v %x: nasName=%v, nasIpAddr=%v, deviceMac=%v, dhcpServerName=%v, dhcpServerId=%v", r.Code.String(), r.Authenticator, nasName, nasIpAddr, deviceMAC, dhcpServerName, dhcpServerId)
	agent := new(events.RequestAgent)
	remoteId := redback_agent_parsers.ParseRemoteId(redback.AgentRemoteID_Get(r.Packet))
	if remoteId == "" && rad.agentParsing {
		prom.ErrorsInc(prom.Warning, "radius")
		rad.lg.WarningF("%v %x: no agent information in request (deviceMac=%v, nasIp=%v, dhcpServerName=%v)", r.Code.String(), r.Authenticator, deviceMAC, nasIpAddr, dhcpServerName)
		agent = nil
	} else {
		agent.RemoteId = remoteId
		rad.lg.DebugF("%v %x: agentRemoteId=%v", r.Code.String(), r.Authenticator, agent.RemoteId)
		if bts := redback.AgentCircuitID_Get(r.Packet); len(bts) > 2 {
			agent.RawCircuitId = fmt.Sprintf("%X", bts[2:])
			if rad.agentParsing {
				circuit, er := redback_agent_parsers.Parse(bts)
				if er != nil {
					prom.ErrorsInc(prom.Error, "radius")
					rad.lg.ErrorF("%v %x: error parse circuitId=%x from remote agentId=%v", r.Code.String(), r.Authenticator, bts, remoteId)
				} else {
					rad.lg.DebugF("%v %x: agentCircuitId=%x", r.Code.String(), r.Authenticator, bts[2:])
					rad.lg.DebugF("%v %x: resultParse circuitId - port=%v, vlanId=%v, moduleNum=%v ", r.Code.String(), r.Authenticator, circuit.Port, circuit.VlanId, circuit.Module)
					agent.Circuit = circuit
				}
			}
		}
	}
	request := events.Request{
		NasIp:          nasIpAddr,
		NasName:        nasName,
		DeviceMac:      deviceMAC,
		DhcpServerName: dhcpServerName,
		DhcpServerId:   dhcpServerId,
		Agent:          agent,
	}
	return request, nil
}

func (rad *Radius) _handlerWriteResponse(response events.Response, w radius.ResponseWriter, r *radius.Request) error {
	r.Attributes = make(radius.Attributes)
	switch response.GetRadiusResponseType() {
	case events.SetPool:
		if err := rfc2869.FramedPool_Set(r.Packet, []byte(response.PoolName)); err != nil {
			prom.ErrorsInc(prom.Error, "radius")
			rad.lg.ErrorF("error generate response packet for pool with poolName=%v", response.PoolName)
		}
	case events.SetIpAddress:
		if err := rfc2865.FramedIPAddress_Set(r.Packet, response.GetIp()); err != nil {
			prom.ErrorsInc(prom.Error, "radius")
			rad.lg.ErrorF("error generate response packet for ip=%v", response.IpAddress)
		}
	default:
		rad.lg.ErrorF("unknown type of response set with id: %v", response.GetRadiusResponseType())
		prom.ErrorsInc(prom.Error, "radius")
		return errors.New(fmt.Sprint("unknown type of response set with id: %v", response.GetRadiusResponseType()))
	}

	if response.LeaseTimeSec != 0 {
		if err := rfc2865.SessionTimeout_Set(r.Packet, rfc2865.SessionTimeout(response.LeaseTimeSec)); err != nil {
			prom.ErrorsInc(prom.Error, "radius")
			rad.lg.ErrorF("error set response SessionTimeOut=%v", response.LeaseTimeSec)
		}
	}
	r.Code = radius.CodeAccessAccept
	rad.lg.DebugF("%v %x: ipAddress='%v', poolName='%v', lease_time='%v'", r.Code, r.Authenticator, response.IpAddress, response.PoolName, response.LeaseTimeSec)

	err := w.Write(r.Packet)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (rad *Radius) _handlerWriteResponseRejected(w radius.ResponseWriter, r *radius.Request) error {
	r.Attributes = make(radius.Attributes)
	r.Code = radius.CodeAccessReject
	rad.lg.DebugF("%v %x: Rejected request'", r.Code, r.Authenticator)
	err := w.Write(r.Packet)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}
