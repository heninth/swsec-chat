package share

import "encoding/json"

type MessagePacket struct {
	//sever Target = ip or 'brodcast', client Target = name or 'server'
	Target string
	Sender string
	Type   string
	Value  string
}

func (m *MessagePacket) ToJSONString() (jsonString []byte, err error) {
	jsonString, err = json.Marshal(m)
	jsonString = append(jsonString, '\n')
	return
}
