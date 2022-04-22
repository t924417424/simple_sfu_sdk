var wsObj = null
var pc = null
const UNKNOW = 0
const JOIN_ROOM = 1
const LEAVE_ROOM = 2
const CANDIDATE = 3
const OFFER = 4
const ANSWER = 5
const RENEGOTIATION = 6

// 这里配置一下stun和trun服务器,防止p2p建立失败之后,通过trun进行转发
var iceConfiguration = {
    'iceServer': [{
            'urls': 'stun:stun.l.google.com:19302'
        },
        {
            'urls': 'turn:numb.viagenie.ca',
            'credential': 'muazkh',
            'username': 'webrtc@live.com'
        }
    ]
}


log = function(msg) {
    let el = document.createElement('p')
    el.innerText = msg
    document.getElementById('log').appendChild(el)
}

navigator.mediaDevices.getDisplayMedia({ video: true, audio: true })
    .then(stream => {
        pc = new RTCPeerConnection(iceConfiguration)
        pc.ontrack = function(event) {
            if (event.track.kind === 'audio') {
                return
            }
            log("收到服务器转发的视频流信息")
            let el = document.createElement(event.track.kind)
                // el.id = event.track.id
            el.srcObject = event.streams[0]
            el.autoplay = true
            el.controls = true
            el.width = 200
                // el.height = 120
            document.getElementById('video_layer').appendChild(el)

            event.track.onmute = function(event) {
                el.play()
            }

            event.streams[0].onremovetrack = ({ track }) => {
                log("移除track事件")
                console.log("remove")
                if (el.parentNode) {
                    el.parentNode.removeChild(el)
                }
            }
        }

        pc.onnegotiationneeded = function() {
            if (wsObj) {
                wsObj.send(JSON.stringify({ event: RENEGOTIATION }))
            }
        }

        pc.onicecandidate = e => {
            console.log("candidate" + e.candidate)
            if (!e.candidate) {
                return
            }

            wsObj.send(JSON.stringify({ event: CANDIDATE, data: JSON.stringify(e.candidate) }))
        }

        document.getElementById('localVideo').srcObject = stream
        console.log(stream)
        stream.getTracks().forEach(track => pc.addTrack(track, stream))

        join_room = function() {
            rid = document.getElementById("rid").value
            uid = document.getElementById("uid").value
            if (rid == "" || uid == "") {
                alert("房间ID和用户ID不能为空")
                return
            }
            init_websocket("ws://127.0.0.1:8081/sfu?rid=" + rid + "&uid=" + uid)
        }

        init_websocket = function(ws_url) {
            console.log("init websocket")
            wsObj = new WebSocket(ws_url)
            wsObj.onopen = function() {
                log("websocket信令服务器已连接")
                console.log("websocket已连接")
            }
            wsObj.onclose = function(e) {
                log("信令服务器断开:" + e)
                console.log(e)
            }
            wsObj.onerror = function(e) {
                log("信令服务器错误" + e)
                console.log(e)
            }
            wsObj.onmessage = function(data) {
                console.log(data)
                let msg = JSON.parse(data.data)
                if (!msg) {
                    return console.log('failed to parse msg')
                }

                switch (msg.event) {
                    case OFFER:
                        let offer = JSON.parse(msg.data)
                        if (!offer) {
                            return console.log('failed to parse answer')
                        }
                        pc.setRemoteDescription(offer)
                        pc.createAnswer().then(answer => {
                            pc.setLocalDescription(answer)
                            wsObj.send(JSON.stringify({ event: ANSWER, data: JSON.stringify(answer) }))
                        })
                        return

                    case CANDIDATE:
                        let candidate = JSON.parse(msg.data)
                        if (!candidate) {
                            return console.log('failed to parse candidate')
                        }

                        pc.addIceCandidate(candidate)
                    case LEAVE_ROOM:
                        log("用户" + msg.data + "离开房间.")
                        console.log("用户" + msg.data + "离开房间.")
                }
            }
        }
    })