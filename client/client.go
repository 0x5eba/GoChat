package client

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	b64 "encoding/base64"

	_ "github.com/go-sql-driver/mysql"

	network "github.com/0x5eba/GoChat/protobufs/build/network"
	"github.com/abiosoft/ishell"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	readOther chan []byte
	wg        sync.WaitGroup
	isOpen    bool
}

var (
	ServerIP   string
	myUsername string
)

type User struct {
	Rsa       string `json:"rsa"`
	B64AesKey string `json:"aeskey"`
	B64AesIV  string `json:"aesiv"`
}

// StartClient start the client
func StartClient(serverIP, username string) {
	ServerIP = serverIP
	myUsername = username
	c, err := ConnectServer(ServerIP, username)
	if err != nil {
		log.Fatal(err)
	}

	publicRsaKeyBytes, err := ioutil.ReadFile("./client/RsaKeys" + myUsername + "/public.pem")
	if err != nil {
		log.Error(err)
	}

	// TODO cambia password
	db, err := sql.Open("mysql", "root:test123@tcp(127.0.0.1:3306)/")
	if err != nil {
		log.Print(err.Error())
	}
	defer db.Close()
	db.Exec("USE Client;")
	db.Exec("SET NAMES utf8mb4;")

	shell := ishell.New()

	shell.AddCmd(&ishell.Cmd{
		Name: "chat",
		Help: "List online users",
		Func: func(shellContent *ishell.Context) {

			shellContent.ShowPrompt(false)
			defer shellContent.ShowPrompt(true)

			chats, err := getUsers(ServerIP)
			if err != nil {
				log.Error(err)
			}

			onlyUsernames := []string{}
			for _, chat := range chats {
				usernameChat := strings.Split(chat, " -> ")[1]
				onlyUsernames = append(onlyUsernames, usernameChat)
			}

			choice := shellContent.MultiChoice(onlyUsernames, "To whom do you want to write?")

			currentChat := strings.Split(chats[choice], " -> ")[0]

			// check if the user already exist in the db
			var resQuery User
			err = db.QueryRow("SELECT * FROM Chats where rsa_key = ?", currentChat).Scan(&resQuery.Rsa, &resQuery.B64AesKey, &resQuery.B64AesIV)

			// if doesn't exist create the channel with a AES key
			if err != nil && resQuery.Rsa == "" {
				msg, aes, err := c.sendAesKey(currentChat)
				if err != nil {
					log.Error(err)
				}

				message := &network.Message{
					Type:        network.Message_AES_CLIENT,
					Data:        msg,
					Destination: []byte(currentChat),
					Sender:      publicRsaKeyBytes,
				}
				messageByte, err := proto.Marshal(message)
				if err != nil {
					log.Error(err)
				}

				env := &network.Envelope{
					Data: messageByte,
					Type: network.Envelope_MESSAGE,
				}
				data, err := proto.Marshal(env)
				if err != nil {
					log.Error(err)
				}

				if c.isOpen {
					c.send <- data
				}

				aesStruct := &network.AES{}
				proto.Unmarshal(aes, aesStruct)

				b64Key := b64.URLEncoding.EncodeToString(aesStruct.GetKey())
				b64IV := b64.URLEncoding.EncodeToString(aesStruct.GetIV())

				resQuery.B64AesKey = b64Key
				resQuery.B64AesIV = b64IV

				insert, err := db.Query("INSERT INTO Chats VALUES ( ?, ?, ? )", currentChat, b64Key, b64IV)
				if err != nil {
					log.Error(err.Error())
				}
				defer insert.Close()
			}

			shellContent.Print("> ")
			messageToSend := shellContent.ReadLine()

			c.sendMessage(messageToSend, []byte(currentChat), publicRsaKeyBytes, resQuery, network.Message_MESSAGE)
		},
	})

	shell.Run()
}

func (c *client) sendMessage(messageToSend string, userPublicRsaKey, publicRsaKeyBytes []byte, user User, typeMessage network.Message_MessageType) {
	var encryptedMessage []byte
	aesKey, _ := b64.URLEncoding.DecodeString(user.B64AesKey)
	aesIV, _ := b64.URLEncoding.DecodeString(user.B64AesIV)

	if strings.HasPrefix(messageToSend, "./") || strings.HasPrefix(messageToSend, "/") || strings.HasPrefix(messageToSend, "~/") {
		var file []byte
		var err error

		file, err = ioutil.ReadFile(messageToSend)
		if err != nil {
			log.Error(err)
		}

		encryptedMessage = EncryptAes(file, aesKey, aesIV)
	} else {
		encryptedMessage = EncryptAes([]byte(messageToSend), aesKey, aesIV)
	}

	message := &network.Message{
		Type:        typeMessage,
		Data:        encryptedMessage,
		Destination: userPublicRsaKey,
		Sender:      publicRsaKeyBytes,
	}
	messageByte, err := proto.Marshal(message)
	if err != nil {
		log.Error(err)
	}

	env := &network.Envelope{
		Data: messageByte,
		Type: network.Envelope_MESSAGE,
	}
	data, err := proto.Marshal(env)
	if err != nil {
		log.Error(err)
	}

	if c.isOpen {
		c.send <- data
	}
}

func (c *client) sendAesKey(userPublicRsaKey string) ([]byte, []byte, error) {
	publicRsaKey := BytesToPublicKey(userPublicRsaKey)

	aesKey, IV, err := CreateKey()
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}

	AES := &network.AES{
		Key: aesKey,
		IV:  IV,
	}
	AESByte, _ := proto.Marshal(AES)

	msg := EncryptWithPublicKey(AESByte, publicRsaKey)
	return msg, AESByte, nil
}

func getUsers(ip string) ([]string, error) {
	peerURL := fmt.Sprintf("http://%s/peers", ip)

	resp, err := http.Get(peerURL)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	allRsaKeys := []string{}
	err = json.Unmarshal(data, &allRsaKeys)
	if err != nil {
		return nil, err
	}

	return allRsaKeys, nil
}

// ConnectServer is to connect to clients
func ConnectServer(ip, username string) (*client, error) {
	dial := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dial.Dial(fmt.Sprintf("ws://%s/ws", ip), nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	c := client{
		conn:      conn,
		send:      make(chan []byte, 256),
		readOther: make(chan []byte, 256),
		isOpen:    true,
	}

	pemString, err := ioutil.ReadFile("./client/RsaKeys" + myUsername + "/public.pem")
	if err != nil {
		log.Error(err)
		return nil, err
	}

	user := &network.User{
		RsaKey:   pemString,
		Username: username,
	}
	userByte, _ := proto.Marshal(user)

	message := &network.Message{
		Type: network.Message_RSA_SERVER,
		Data: userByte,
	}
	messageByte, _ := proto.Marshal(message)

	env := &network.Envelope{
		Type: network.Envelope_MESSAGE,
		Data: messageByte,
	}
	data, _ := proto.Marshal(env)

	if c.isOpen {
		c.send <- data
	}

	go c.read()
	go c.write()

	return &c, nil
}

// read reads data from the socket and handles it
func (c *client) read() {
	// Unregister if the node dies
	defer func() {
		log.Info("Client/Server died")
		c.isOpen = false
		c.conn.Close()
	}()

	privateRsaKeyBytes, err := ioutil.ReadFile("./client/RsaKeys" + myUsername + "/private.pem")
	if err != nil {
		log.Error(err)
	}
	publicRsaKeyBytes, err := ioutil.ReadFile("./client/RsaKeys" + myUsername + "/public.pem")
	if err != nil {
		log.Error(err)
	}
	publicRsaKey := BytesToPublicKey(string(publicRsaKeyBytes))
	privateRsaKey := BytesToPrivateKey(string(privateRsaKeyBytes))

	log.Info("This is my public RSA key: ", b64.URLEncoding.EncodeToString(publicRsaKeyBytes))

	for {
		msgType, msg, err := c.conn.ReadMessage()
		if err != nil {
			continue
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		pb := network.Envelope{}
		err = proto.Unmarshal(msg, &pb)
		if err != nil {
			log.Error(err)
			continue
		}

		go func() {
			switch pb.GetType() {
			case network.Envelope_MESSAGE:
				message := network.Message{}
				err = proto.Unmarshal(pb.GetData(), &message)
				if err != nil {
					log.Error(err)
				}

				if reflect.DeepEqual(message.GetDestination(), publicRsaKey) == true {
					return
				}

				db, err := sql.Open("mysql", "root:test123@tcp(127.0.0.1:3306)/")
				if err != nil {
					log.Print(err.Error())
				}
				defer db.Close()
				db.Exec("USE Client;")
				db.Exec("SET NAMES utf8mb4;")

				switch message.GetType() {
				case network.Message_AES_CLIENT:
					log.Info("Raw message: ", string(message.GetData()))
					// decrypt rsa and get the aes key
					aesKey := DecryptWithPrivateKey(message.GetData(), privateRsaKey)
					var aesKeyStruct network.AES
					proto.Unmarshal(aesKey, &aesKeyStruct)

					// save the key in db
					b64Key := b64.URLEncoding.EncodeToString(aesKeyStruct.GetKey())
					b64IV := b64.URLEncoding.EncodeToString(aesKeyStruct.GetIV())
					insert, err := db.Query("INSERT INTO Chats VALUES ( ?, ?, ? )", string(message.GetSender()), b64Key, b64IV)
					if err != nil {
						log.Error(err.Error())
					}
					defer insert.Close()

					log.Info("AES Keys:\n", b64Key, "\n", b64IV)

				case network.Message_MESSAGE:
					chats, err := getUsers(ServerIP)
					if err != nil {
						log.Error(err)
					}

					usernameSender := "[]"
					for _, chat := range chats {
						if reflect.DeepEqual([]byte(strings.Split(chat, " -> ")[0]), message.GetSender()) == true {
							usernameSender = "[" + strings.Split(chat, " -> ")[1] + "]"
							break
						}
					}

					var resQuery User
					err = db.QueryRow("SELECT * FROM Chats where rsa_key = ?", message.GetSender()).Scan(&resQuery.Rsa, &resQuery.B64AesKey, &resQuery.B64AesIV)
					if err != nil {
						log.Error(err.Error())
					}

					if err == nil {
						aesKey, _ := b64.URLEncoding.DecodeString(resQuery.B64AesKey)
						aesIV, _ := b64.URLEncoding.DecodeString(resQuery.B64AesIV)
						senderMessage := DecryptAes(message.GetData(), aesKey, aesIV)

						// TODO check the extension

						log.Println(usernameSender, ":", string(senderMessage))
					}

				}
			}
		}()
	}
}

// write checks the channel for data to write and writes it to the socket
func (c *client) write() {
	for {
		toWrite := <-c.send
		_ = c.conn.WriteMessage(websocket.BinaryMessage, toWrite)
	}
}
