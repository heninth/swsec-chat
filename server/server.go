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
var clientBox *walk.ListBox
var messageInput *walk.LineEdit
var sendBtn *walk.PushButton
var kickBtn *walk.PushButton

type chatMessage struct {
	Name    string
	Message string
}

var clientList = make(map[string]*share.ClientInfo, 32) //key = ip
var chatHistory []chatMessage

var proxyConnection = make(map[string]*net.TCPConn, 8) //key = ip

func main() {
	MainWindow{
		AssignTo: &window,
		Title:    "Server",
		MinSize:  Size{600, 400},
		MaxSize:  Size{600, 400},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					TextEdit{
						AssignTo: &chatBox,
						ReadOnly: true,
						VScroll:  true,
						MinSize:  Size{300, 0},
					},
					Composite{
						Layout: VBox{MarginsZero: true},
						Children: []Widget{
							ListBox{
								AssignTo: &clientBox,
								Model:    &share.ClientModel{Items: make([]*share.ClientInfo, 0)},
								OnCurrentIndexChanged: func() {
									if clientBox.CurrentIndex() < 0 {
										kickBtn.SetEnabled(false)
									} else {
										kickBtn.SetEnabled(true)
									}
								},
							},
							PushButton{
								AssignTo:  &kickBtn,
								Text:      "Kick",
								Enabled:   false,
								OnClicked: kickClient,
							},
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
						OnClicked: serverAnnounce,
					},
				},
			},
		},
	}.Create()

	go func() { //proxy listener
		proxyListener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 6701})
		if err != nil {
			panic(err)
		} else {
			chatBox.AppendText("Proxy listening on port 6701\r\n")

			go func() { // proxy on connected
				for {
					conn, err := proxyListener.AcceptTCP() //on connected
					if err != nil {
						fmt.Println("[proxy]" + err.Error())
					} else {
						fmt.Println("Proxy connect " + conn.RemoteAddr().String())

						go func(conn *net.TCPConn) { //proxy hendeler
							proxyConnection[conn.RemoteAddr().String()] = conn

							for {
								var buffer []byte
								temp := make([]byte, 1)
								packetLen := 0

								for {
									_, err := conn.Read(temp)

									if err != nil { //disconnected
										delete(proxyConnection, conn.RemoteAddr().String())
										conn.Close()
										return
									}

									if temp[0] == '\n' {
										break
									} else {
										buffer = append(buffer, temp[0])
										packetLen++
									}
								}
								//already trim ":Chat" from another server
								message := strings.Split(string(buffer), ":")

								//check is cross server priate message
								pm := strings.Split(message[1], ";")
								if len(pm) == 2 {
									if _, ok := clientList[_getIPByName(pm[0])]; ok { //check is target exist in client list
										messagePacket := share.MessagePacket{
											Target: clientList[_getIPByName(pm[0])].Conn.RemoteAddr().String(),
											Sender: message[0],
											Type:   "private_message",
											Value:  pm[1],
										}
										_sendMessagePacket(messagePacket, false)
									}
								} else { //not private message
									_addChatBoxMessage(chatMessage{
										message[0],
										message[1],
									}, true, false)
								}
							}
						}(conn)
					}
				}
			}()
		}
	}()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 6700}) //listen
	if err != nil {
		panic(err)
	} else {
		chatBox.AppendText("Listening on port 6700\r\n")

		go func() {
			for {
				conn, err := listener.AcceptTCP() //on connected
				if err != nil {
					walk.MsgBox(window, "Client connection error", err.Error(), walk.MsgBoxIconError)
				} else {
					go connectionHandeler(conn) //handler
				}
			}
		}()
	}

	window.Run()
}

func connectionHandeler(conn *net.TCPConn) {
	_addChatBoxMessage(chatMessage{
		"server",
		conn.RemoteAddr().String() + " Connected",
	}, false, false)

	for {
		var buffer []byte
		temp := make([]byte, 1)
		packetLen := 0

		for {
			_, err := conn.Read(temp)

			if err != nil { //disconnected
				_addChatBoxMessage(chatMessage{
					"server",
					conn.RemoteAddr().String() + " Disconnected",
				}, false, false)
				delete(clientList, conn.RemoteAddr().String())
				updateClientList()
				conn.Close()
				return
			}

			if temp[0] == '\n' {
				break
			} else {
				buffer = append(buffer, temp[0])
				packetLen++
			}
		}

		fmt.Println("Receive : " + string(buffer))

		var messagePacket share.MessagePacket
		err := json.Unmarshal(buffer, &messagePacket)

		if err != nil {
			fmt.Println(err.Error())
		} else {
			if messagePacket.Type == "init" {
				addClient(conn, messagePacket.Value)

			} else {
				if _, ok := clientList[conn.RemoteAddr().String()]; !ok {
					return
				}

				switch messagePacket.Type {
				case "message":
					//check is it private message by command
					//"NAME; MESSAGE"
					data := strings.Split(messagePacket.Value, ";")
					if len(data) == 2 {
						if _, ok := clientList[_getIPByName(data[0])]; ok {
							//client on this server
							_sendMessagePacket(share.MessagePacket{
								Sender: messagePacket.Sender,
								Target: clientList[_getIPByName(data[0])].Conn.RemoteAddr().String(),
							}, false)
						} else {
							//fw to another server
							for _, client := range proxyConnection {
								client.Write([]byte(messagePacket.Sender + ":" + messagePacket.Value + "\n"))
							}
						}
					}

					//add message from client to server's message box
					//forward message to another client
					_addChatBoxMessage(chatMessage{
						clientList[conn.RemoteAddr().String()].Name,
						messagePacket.Value,
					}, true, true)

					for i := range proxyConnection {
						proxyConnection[i].Write([]byte(clientList[conn.RemoteAddr().String()].Name + ":" + messagePacket.Value))
					}

				case "private_message": //send private message from ui, so it's can only send to our server
					if _, ok := clientList[_getIPByName(messagePacket.Target)]; ok { //check is target exist in client list
						buffer = append(buffer, '\n')
						clientList[_getIPByName(messagePacket.Target)].Conn.Write(buffer) //fw
						fmt.Println("Send : " + string(buffer))
					} else {
						messagePacket := share.MessagePacket{
							Target: conn.RemoteAddr().String(),
							Sender: "server",
							Type:   "error",
							Value:  "Can't send message to" + messagePacket.Target,
						}
						_sendMessagePacket(messagePacket, false)
					}
				}
			}
		}
	}
}

func addClient(conn *net.TCPConn, name string) {
	for _, client := range clientList {
		if client.Name == name {
			messagePacket := share.MessagePacket{
				Target: conn.RemoteAddr().String(),
				Sender: "server",
				Type:   "error",
				Value:  "Duplicate nickname",
			}
			jsonString, err := messagePacket.ToJSONString()

			if err != nil {
				walk.MsgBox(window, "Error", err.Error(), walk.MsgBoxIconError)
			} else {
				conn.Write(jsonString)
				fmt.Println("Send : " + string(jsonString))
			}
			conn.Close()
			return
		}
	}

	clientList[conn.RemoteAddr().String()] = &share.ClientInfo{
		Conn: conn,
		Name: name,
	}
	updateClientList()
}

func serverAnnounce() {
	message := messageInput.Text()
	message = strings.Replace(message, "\n", "", 1)
	message = strings.Replace(message, "\r", "", 1)
	messageInput.SetText("")

	if strings.Replace(message, " ", "", -1) == "" {
		return
	}

	_addChatBoxMessage(chatMessage{
		"server",
		message,
	}, true, true)
}

func kickClient() {
	client := clientBox.Model().(*share.ClientModel).Items[clientBox.CurrentIndex()]
	clientAddress := client.Conn.RemoteAddr().String()

	messagePacket := share.MessagePacket{
		Target: clientAddress,
		Sender: "server",
		Type:   "kick",
		Value:  "",
	}
	_sendMessagePacket(messagePacket, false)
	_addChatBoxMessage(chatMessage{
		"server",
		"kick " + client.Name,
	}, true, false)
	client.Conn.Close()
}

func updateClientList() {
	var list []*share.ClientInfo
	for i := range clientList {
		list = append(list, clientList[i])
	}
	err := clientBox.SetModel(&share.ClientModel{Items: list})
	if err != nil {
		walk.MsgBox(window, "Error", err.Error(), walk.MsgBoxIconError)
	} else {
		var nameList []string
		for i := range list {
			nameList = append(nameList, list[i].Name)
		}
		jsonString, err := json.Marshal(nameList)

		if err != nil {
			walk.MsgBox(window, "Error", err.Error(), walk.MsgBoxIconError)
		} else {
			messagePacket := share.MessagePacket{
				Target: "brodcast",
				Sender: "server",
				Type:   "update_client",
				Value:  string(jsonString),
			}
			_sendMessagePacket(messagePacket, false)
		}
	}
}

func _addChatBoxMessage(message chatMessage, brodcast bool, isProxyMessage bool) {
	chatBox.AppendText(message.Name + " : " + message.Message + "\r\n")
	chatHistory = append(chatHistory, message)
	if brodcast {
		_sendMessagePacket(share.MessagePacket{
			Target: "brodcast",
			Sender: message.Name,
			Type:   "message",
			Value:  message.Message,
		}, isProxyMessage)
	}
}

func _sendMessagePacket(messagePacket share.MessagePacket, isProxyMessage bool) {
	jsonString, err := messagePacket.ToJSONString()
	if err != nil {
		walk.MsgBox(window, "Error", err.Error(), walk.MsgBoxIconError)
	} else {
		fmt.Println("Send : " + string(jsonString))

		if messagePacket.Target == "brodcast" {
			for _, client := range clientList {
				client.Conn.Write(jsonString)
			}

			if isProxyMessage {
				for _, client := range proxyConnection {
					client.Write([]byte(messagePacket.Sender + ":" + messagePacket.Value + "\n"))
				}
			}

		} else {
			if _, ok := clientList[messagePacket.Target]; ok {
				clientList[messagePacket.Target].Conn.Write(jsonString)
			} else {
				walk.MsgBox(window, "Error", "No key '"+messagePacket.Target+"' in 'clientList'", walk.MsgBoxIconError)
			}
		}
	}
}

func _getIPByName(name string) string {
	for i := range clientList {
		if clientList[i].Name == name {
			return clientList[i].Conn.RemoteAddr().String()
		}
	}
	return ""
}
