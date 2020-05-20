package redback_agent_parsers

import (
	"encoding/binary"
	"errors"
)

type CircuitId struct {
	VlanId int `json:"vlan_id"`
	Module int `json:"module"`
	Port   int `json:"port"`
}

func Parse(cb []byte) (*CircuitId, error) {
	switch len(cb) {
	case 6:
		return DlinkAgentCircuitParser(cb)
	default:
		return nil, errors.New("not support type of circuitId")
	}
}

//INPUT - [0 4 0 101 0 3] влан - 101, порт 3
//INPUT - [0 4 3 242 0 3] влан - 1010, порт 3
// [0] - Тип опции
// [4] - Длина
// [0 101] - VLAN ID DHCP-запроса клиента
// [0] - Module : Для автономного коммутатора, поле Module всегда 0; для стекируемого коммутатора, Module = Unit ID.
// [3] - Порт
func DlinkAgentCircuitParser(cb []byte) (*CircuitId, error) {
	if len(cb) != 6 {
		return nil, errors.New("error parse circuitId for dlink, len must be 6 bytes")
	}

	vlanId, err := convertByteSliceToInt(cb[2:4])
	if err != nil {
		return nil, errors.New("error parse vlan from circuitId")
	}
	return &CircuitId{
		VlanId: vlanId,
		Module: int(cb[4]),
		Port:   int(cb[5]),
	}, nil
}

func convertByteSliceToInt(slice []byte) (int, error) {
	if len(slice) > 4 {
		return 0, errors.New("cannot convert to int - oversize")
	}
	slice = append(make([]byte, 4-len(slice)), slice...)
	return int(binary.BigEndian.Uint32(slice)), nil
}
