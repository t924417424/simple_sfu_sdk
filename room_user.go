package simple_sfu

import (
	"sync"

	"github.com/pion/webrtc/v3"
)

type signalPeerConnectionsFunc func()
type OfferEventFunc func(offer webrtc.SessionDescription) error
type OnICECandidateFunc func(i *webrtc.ICECandidate)

type userInfo struct {
	r    *sync.RWMutex
	uid  string
	peer *webrtc.PeerConnection
	// 用户媒体轨道
	trackLocals           map[string]*webrtc.TrackLocalStaticRTP
	signalPeerConnections signalPeerConnectionsFunc
	offerEvent            OfferEventFunc
	onICECandidate        OnICECandidateFunc
}

func newUserInfo(uid string, peer *webrtc.PeerConnection, SignalPeerConnections signalPeerConnectionsFunc, offerEvent OfferEventFunc, onICECandidate OnICECandidateFunc) *userInfo {
	return &userInfo{r: &sync.RWMutex{}, uid: uid, peer: peer, signalPeerConnections: SignalPeerConnections, trackLocals: make(map[string]*webrtc.TrackLocalStaticRTP), offerEvent: offerEvent, onICECandidate: onICECandidate}
}

// func (u *userInfo) listenOfferEvent(offer webrtc.SessionDescription) {
// 	if u.offerEvent == nil {
// 		log.Printf("[Error] 用户 %s 没有注册offer事件,将无法收到服务端offer进行连接建立!", u.uid)
// 		return
// 	}
// 	u.offerEvent(offer)
// }

func (u *userInfo) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	return u.peer.AddICECandidate(candidate)
}

func (u *userInfo) SetRemoteDescription(desc webrtc.SessionDescription) error {
	return u.peer.SetRemoteDescription(desc)
}

func (u *userInfo) close() {
	u.r.Lock()
	defer u.r.Unlock()
	u.peer.Close()
}

func (u *userInfo) setTrack(t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	u.r.Lock()
	defer u.r.Unlock()
	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		panic(err)
	}
	// log.Printf("recv remote user %s trackId %s", u.uid, t.ID())
	u.trackLocals[t.ID()] = trackLocal
	return trackLocal
}

func (u *userInfo) removeTrack(track *webrtc.TrackLocalStaticRTP) {
	u.trackLocals = nil
}
