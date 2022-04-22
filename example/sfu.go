package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	simple_sfu "github.com/t924417424/simple_sfu_sdk"
)

type msgEvent int

const (
	UNKNOW msgEvent = iota
	JOIN_ROOM
	LEAVE_ROOM
	CANDIDATE
	OFFER
	ANSWER
	RENEGOTIATION
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	// 用户保存房间内的用户信息,方便实现消息广播等业务
	roomUser = make(map[string]map[string]*conns)
)

type conns struct {
	uid  string
	conn *websocket.Conn
}

type WsMsg struct {
	Event msgEvent `json:"event"`
	Data  string   `json:"data"`
}

func main() {
	http.HandleFunc("/sfu", testSfu)
	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("example/static/"))))
	http.ListenAndServe(":8081", nil)
}

func testSfu(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		w.Write([]byte(err.Error()))
		return
	}
	defer conn.Close()
	rid := r.URL.Query().Get("rid")
	if rid == "" {
		_ = conn.WriteJSON(&WsMsg{
			Event: UNKNOW,
			Data:  "房间号参数[rid]错误",
		})
		return
	}
	uid := r.URL.Query().Get("uid")
	if rid == "" {
		_ = conn.WriteJSON(&WsMsg{
			Event: UNKNOW,
			Data:  "用户id参数[uid]错误",
		})
		return
	}
	room := simple_sfu.FindRoom(rid)
	user, leaveFn, err := room.JoinRoom(uid, func(offer webrtc.SessionDescription) error {
		// 当服务端创建了offer,会进入这个函数
		offerString, err := json.Marshal(offer)
		if err != nil {
			return err
		}
		if err = conn.WriteJSON(&WsMsg{
			Event: OFFER,
			Data:  string(offerString),
		}); err != nil {
			return err
		}
		return nil
	}, func(i *webrtc.ICECandidate) {
		// 当收产生ICECandidate会进入这个函数
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Println(err)
			return
		}

		if writeErr := conn.WriteJSON(&WsMsg{
			Event: CANDIDATE,
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Println(writeErr)
		}
	})
	if err != nil {
		_ = conn.WriteJSON(&WsMsg{
			Event: UNKNOW,
			Data:  "用户加入房间失败" + err.Error(),
		})
	}

	defer func() {
		// 注意调用这个函数,内部逻辑为当最后一个用户离开房间,则关闭房间
		leaveFn()
		// 向房间内所有用户广播某用户离开,这里可以做一下房间删除逻辑
		for i := range roomUser[rid] {
			_ = roomUser[rid][i].conn.WriteJSON(&WsMsg{
				Event: LEAVE_ROOM,
				Data:  uid,
			})
		}
	}()
	// 初始化二级map
	if roomUser[rid] == nil {
		roomUser[rid] = make(map[string]*conns)
	}
	roomUser[rid][uid] = &conns{uid: uid, conn: conn}
	message := &WsMsg{}
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		} else if err := json.Unmarshal(raw, &message); err != nil {
			log.Println(err)
			return
		}
		switch message.Event {
		case LEAVE_ROOM:
			return
		case CANDIDATE:
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				log.Println(err)
				return
			}

			if err := user.AddICECandidate(candidate); err != nil {
				log.Println(err)
				return
			}
		case ANSWER:
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				log.Println(err)
				return
			}

			if err := user.SetRemoteDescription(answer); err != nil {
				log.Println(err)
				return
			}
		case RENEGOTIATION:
			room.SignalPeerConnections()
		}

	}
}
