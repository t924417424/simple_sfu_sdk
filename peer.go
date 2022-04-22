package simple_sfu

import (
	"log"

	"github.com/pion/webrtc/v3"
)

func initPeer(u *userInfo) (err error) {
	// 获取用户peer
	peerConnection := u.peer
	// 媒体接收器
	// 为peer添加音频和视频接收器,这个peer将接受一个音频和一个视频
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		log.Print(err)
		return err
	}
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		log.Print(err)
		return err
	}
	// 媒体接收器

	// track事件,从客户端track创建服务的track,用于转发
	peerConnection.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		trackLocal := u.setTrack(tr)
		defer u.remoteTrack(trackLocal)
		buf := make([]byte, 1500)
		u.signalPeerConnections()
		for {
			i, _, err := tr.Read(buf)
			if err != nil {
				return
			}

			if _, err = trackLocal.Write(buf[:i]); err != nil {
				return
			}
		}
	})

	// peer状态改变事件
	u.peer.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		switch p {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				log.Print(err)
			}
		case webrtc.PeerConnectionStateClosed:
			u.signalPeerConnections()
		case webrtc.PeerConnectionStateConnected:
			u.signalPeerConnections()
		}
	})

	return nil
}
