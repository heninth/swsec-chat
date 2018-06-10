package share

import (
	"net"

	"github.com/lxn/walk"
)

type ClientInfo struct {
	Conn *net.TCPConn
	Name string
}

type ClientModel struct {
	walk.ListModelBase
	Items []*ClientInfo
}

func (m *ClientModel) ItemCount() int {
	return len(m.Items)
}

func (m *ClientModel) Get(index int) *ClientInfo {
	return m.Items[index]
}

func (m *ClientModel) Value(index int) interface{} {
	if m.Items[index].Conn == nil {
		return m.Items[index].Name
	}
	return m.Items[index].Name + " (" + m.Items[index].Conn.RemoteAddr().String() + ")"
}
