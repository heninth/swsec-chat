package main

import (
	"encoding/json"
	"fmt"
	"net"
	"software-sec-project/share"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

//walk
var window *walk.MainWindow
var chatBox *walk.TextEdit
var nameInput *walk.LineEdit
var serverIPInput *walk.LineEdit
var connectBtn *walk.PushButton
var messageInput *walk.LineEdit
var sendBtn *walk.PushButton
var sendPmBtn *walk.PushButton
var clientBox *walk.ListBox

var conn *net.TCPConn

func main() {
	MainWindow{
		AssignTo: &window,
		Title:    "Client",
		MinSize:  Size{600, 400},
		MaxSize:  Size{600, 400},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{
						Text: "Nickname : ",
					},
					LineEdit{
						AssignTo: &nameInput,
						Text:     "",
					},
					Label{
						Text: "Server(IP:PORT) : ",
					},
					LineEdit{
						AssignTo: &serverIPInput,
						Text:     "127.0.0.1:6700",
					},
					PushButton{
						AssignTo:  &connectBtn,
						Text:      "Connect",
						OnClicked: connectServer,
					},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					TextEdit{
						AssignTo: &chatBox,
						ReadOnly: true,
						VScroll:  true,
						MinSize:  Size{300, 0},
					},
					ListBox{
						AssignTo: &clientBox,
						Model:    &share.ClientModel{Items: make([]*share.ClientInfo, 0)},
						OnCurrentIndexChanged: func() {
							if clientBox.CurrentIndex() < 0 {
								sendPmBtn.SetEnabled(false)
							} else {
								if clientBox.Model().(*share.ClientModel).Items[clientBox.CurrentIndex()].Name == nameInput.Text() {
									sendPmBtn.SetEnabled(false)
								} else {
									sendPmBtn.SetEnabled(true)
								}
							}
						},
					},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{
						AssignTo:  &messageInput,
						MaxLength: 512,
					},
					PushButton{
						AssignTo:  &sendBtn,
						Text:      "Send",
						Enabled:   false,
						OnClicked: sendMessage,
					},
					PushButton{
						AssignTo:  &sendPmBtn,
						Text:      "Send PM",
						Enabled:   false,
						OnClicked: sendPmMessage,
					},
				},
			},
		},
	}.Run()
}

func connectServer() {
	if strings.Replace(nameInput.Text(), " ", "", -1) == "" {
		return
	}

	//parse server addr
	serverAddrString := strings.Replace(serverIPInput.Text(), "\n", "", 1)
	serverAddrString = strings.Replace(serverAddrString, "\r", "", 1)
	serverAddr, err := net.ResolveTCPAddr("tcp", serverAddrString)
	if err != nil {
		walk.MsgBox(window, "Cannot resolve server address", err.Error(), walk.MsgBoxIconError)
	} else {
		//connect
		_conn, err := net.DialTCP("tcp", nil, serverAddr)
		conn = _conn
		if err != nil {
			fmt.Println(err)
			walk.MsgBox(window, "Cannot connect to server", err.Error(), walk.MsgBoxIconError)
		} else {
			messagePacket := share.MessagePacket{
				Target: "server",
				Sender: conn.LocalAddr().String(),
				Type:   "init",
				Value:  nameInput.Text(),
			}
			jsonString, err := messagePacket.ToJSONString()

			if err != nil {
				walk.MsgBox(window, "Message error", err.Error(), walk.MsgBoxIconError)
			} else {
				conn.Write(jsonString) //send
				fmt.Println("Send : " + string(jsonString))

				chatBox.AppendText("Connect to " + serverAddrString + "\r\n")
				sendBtn.SetEnabled(true)
				nameInput.SetEnabled(false)
				serverIPInput.SetEnabled(false)
				connectBtn.SetEnabled(false)
				go serverHandeler()
			}
		}
	}
}

func serverHandeler() {
	defer closeConnection()
	for {
		var buffer []byte
		temp := make([]byte, 1)
		len := 0
		for {
			_, err := conn.Read(temp)

			if err != nil { //disconnected
				return
			}

			if temp[0] == '\n' {
				break
			} else {
				buffer = append(buffer, temp[0])
				len++
			}
		}

		fmt.Println("Receive : " + string(buffer[:len]))

		var messageJSON share.MessagePacket
		err := json.Unmarshal(buffer[:len], &messageJSON)
		if err != nil {
			walk.MsgBox(window, "Error (Unmarshal share.MessagePacket)", err.Error(), walk.MsgBoxIconError)
			fmt.Println(err.Error())
			fmt.Println(string(buffer[:len]))
		} else {
			switch messageJSON.Type {
			case "message":
				if messageJSON.Sender != conn.LocalAddr().String() {
					chatBox.AppendText(messageJSON.Sender + " : " + messageJSON.Value + "\r\n")
				}

			case "kick":
				chatBox.AppendText("You were kicked from server" + "\r\n")
				return

			case "update_client":
				var clientList []string
				err = json.Unmarshal([]byte(messageJSON.Value), &clientList)
				if err != nil {
					walk.MsgBox(window, "Error (Unmarshal []byte)", err.Error(), walk.MsgBoxIconError)
					fmt.Println(err.Error())
					fmt.Println(messageJSON.Value)
				} else {
					updateClientList(clientList)
				}

			case "private_message":
				chatBox.AppendText(messageJSON.Sender + " -> me : " + messageJSON.Value + "\r\n")

			case "error":
				chatBox.AppendText("Error! " + messageJSON.Value + "\r\n")
			}
		}
	}
}

func updateClientList(clientList []string) {
	var list []*share.ClientInfo
	for i := range clientList {
		list = append(list, &share.ClientInfo{
			Name: clientList[i],
		})
	}
	err := clientBox.SetModel(&share.ClientModel{Items: list})
	if err != nil {
		panic(err)
	}
}

func sendMessage() {
	message := messageInput.Text()
	if strings.Replace(message, " ", "", -1) == "" {
		return
	}

	message = strings.Replace(message, "\n", "", 1)
	message = strings.Replace(message, "\r", "", 1)
	messagePacket := share.MessagePacket{
		Target: "server",
		Sender: nameInput.Text(),
		Type:   "message",
		Value:  message,
	}
	jsonString, err := messagePacket.ToJSONString()
	if err != nil {
		walk.MsgBox(window, "Message error", err.Error(), walk.MsgBoxIconError)
	} else {
		conn.Write(jsonString) //send
		fmt.Println("Send : " + string(jsonString))
		//chatBox.AppendText("me : " + message + "\r\n")
		messageInput.SetText("")
	}
}

func sendPmMessage() {
	target := clientBox.Model().(*share.ClientModel).Items[clientBox.CurrentIndex()]
	message := messageInput.Text()
	if strings.Replace(message, " ", "", -1) == "" {
		return
	}

	message = strings.Replace(message, "\n", "", 1)
	message = strings.Replace(message, "\r", "", 1)
	messagePacket := share.MessagePacket{
		Target: target.Name,
		Sender: nameInput.Text(),
		Type:   "private_message",
		Value:  message,
	}
	jsonString, err := messagePacket.ToJSONString()
	if err != nil {
		walk.MsgBox(window, "Message error", err.Error(), walk.MsgBoxIconError)
	} else {
		conn.Write(jsonString) //send
		fmt.Println("Send : " + string(jsonString))
		chatBox.AppendText("me -> " + target.Name + " : " + message + "\r\n")
		messageInput.SetText("")
	}
}

func closeConnection() {
	conn.Close()
	clientBox.SetModel(&share.ClientModel{Items: make([]*share.ClientInfo, 0)})
	nameInput.SetEnabled(true)
	sendBtn.SetEnabled(false)
	sendPmBtn.SetEnabled(false)
	serverIPInput.SetEnabled(true)
	connectBtn.SetEnabled(true)
	chatBox.AppendText("Disconnected\r\n")
}
