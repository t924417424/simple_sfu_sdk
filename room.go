package simple_sfu

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type userleave func()

var (
	rooms *roomMap = newRoomMap()

	// peer config,配置stun/trun服务器,因服务端一般不会用到trun进行中转,所以服务的只配置stun即可
	peerConfig = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
)

// 保存所有房间
type roomMap struct {
	r     *sync.RWMutex
	rooms map[string]*roomInfo
}

func newRoomMap() *roomMap {
	return &roomMap{r: &sync.RWMutex{}, rooms: make(map[string]*roomInfo)}
}

// 单个房间信息
type roomInfo struct {
	r      *sync.RWMutex
	roomId string
	users  map[string]*userInfo
}

func newRoom(id string) *roomInfo {
	return &roomInfo{r: &sync.RWMutex{}, roomId: id, users: make(map[string]*userInfo)}
}

// 寻找房间，如果不存在则创建,返回一个房间对象
func FindRoom(roomId string) *roomInfo {
	rooms.r.Lock()
	defer rooms.r.Unlock()
	if room, ok := rooms.rooms[roomId]; ok {
		return room
	}
	r := newRoom(roomId)
	rooms.rooms[roomId] = r
	return r
}

func (r *roomInfo) addUser(u *userInfo) *userInfo {
	r.r.Lock()
	defer r.r.Unlock()
	r.users[u.uid] = u
	return r.users[u.uid]
}

// 用户加入房间,参数是用户唯一标识
func (r *roomInfo) JoinRoom(uid string, offerEvent OfferEventFunc, onICECandidate OnICECandidateFunc) (*userInfo, userleave, error) {
	// 创建一个webrtc peer,用于和单个用户建立p2p连接
	peerConnection, err := webrtc.NewPeerConnection(peerConfig)
	if err != nil {
		return nil, nil, err
	}
	user := newUserInfo(uid, peerConnection, r.SignalPeerConnections, offerEvent, onICECandidate)
	// 设置peer相关的方法,初始化音视频接收器
	_ = initPeer(user)
	leaveFunc := func() {
		user.close()
		rooms.r.Lock()
		defer rooms.r.Unlock()
		delete(r.users, uid)
		// 如果房间内不存在用户了,那么就删除房间
		if len(r.users) == 0 {
			delete(rooms.rooms, r.roomId)
		}
	}
	_ = r.addUser(user)
	r.SignalPeerConnections()
	return user, leaveFunc, nil
}

// 当房间中的track或者用户发生改变的时候,将流重新同步
func (r *roomInfo) SignalPeerConnections() {
	r.r.Lock()
	defer func() {
		r.r.Unlock()
		r.dispatchKeyFrame()
	}()

	attemptSync := func() (tryAgain bool) {
		// 删除peer状态为已关闭的用户
		for i := range r.users {
			if r.users[i].peer.ConnectionState() == webrtc.PeerConnectionStateClosed {
				delete(r.users, i)
				return true
			}

			// map of sender we already are seanding, so we don't double send
			existingSenders := map[string]bool{}

			// 检擦房间内所有用户的sender
			// log.Println(r.peerConnections[i].user.uid, "sender=", r.peerConnections[i].user.peerConnection.GetSenders())
			for _, sender := range r.users[i].peer.GetSenders() {
				if sender.Track() == nil {
					continue
				}
				// log.Printf("%s sender trackId %s", r.users[i].uid, sender.Track().ID())
				// log.Println(r.users[i].trackLocals)
				existingSenders[sender.Track().ID()] = true

				// 如果这个sender属于哪个用户推送的track,如果所有用户都不持有这个sender,那么它就是个无效的发送器,则从peer中将其移除
				var isRemove = true
				for j := range r.users {
					if _, ok := r.users[j].trackLocals[sender.Track().ID()]; ok {
						isRemove = false
						break
					}
				}
				if isRemove {
					if err := r.users[i].peer.RemoveTrack(sender); err != nil {
						return true
					}
				}

			}
			// log.Println(r.peerConnections[i].user.uid, "receiver=", r.peerConnections[i].user.peerConnection.GetReceivers())
			// 不转发用户自身推送的流
			for _, receiver := range r.users[i].peer.GetReceivers() {
				if receiver.Track() == nil {
					continue
				}

				existingSenders[receiver.Track().ID()] = true
			}

			// 将peer中还未接收的房间内其他用户的轨道信息添加到这个用户的peer中
			for j := range r.users {
				for trackID := range r.users[j].trackLocals {
					if _, ok := existingSenders[trackID]; !ok {
						if _, err := r.users[i].peer.AddTrack(r.users[j].trackLocals[trackID]); err != nil {
							return true
						}
					}
				}
			}
			offer, err := r.users[i].peer.CreateOffer(nil)
			if err != nil {
				return true
			}

			if err = r.users[i].peer.SetLocalDescription(offer); err != nil {
				return true
			}

			// 将offer信息发送到用户的回调函数中
			if err = r.users[i].offerEvent(offer); err != nil {
				return true
			}
		}

		return
	}

	for syncAttempt := 0; ; syncAttempt++ {
		if syncAttempt == 25 {
			// Release the lock and attempt a sync in 3 seconds. We might be blocking a RemoveTrack or AddTrack
			go func() {
				time.Sleep(time.Second * 3)
				r.SignalPeerConnections()
			}()
			return
		}

		if !attemptSync() {
			break
		}
	}
}

// 向房间内所有用户发送关键帧
func (r *roomInfo) dispatchKeyFrame() {
	r.r.Lock()
	defer r.r.Unlock()

	for i := range r.users {
		for _, receiver := range r.users[i].peer.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}
			_ = r.users[i].peer.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}
